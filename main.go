package main

import (
	"fmt"
	"time"
	"sync"
	"log"
	"github.com/kgolding/go-pocsagencode"
	"github.com/RPIZeroDuplexPOCSAG/rfm69"

	"github.com/davecheney/gpio"
	"github.com/streadway/amqp"
	"github.com/sirius1024/go-amqp-reconnect/rabbitmq"


	"github.com/RPIZeroDuplexPOCSAG/pocsag-gateway/settings"
)
var (
	conf  *settings.App
	queueMutex sync.Mutex
)
var messageQueue []*pocsagencode.Message
var lastTransmitBatch int64

func queueMessage(msg *pocsagencode.Message) {
	queueMutex.Lock()
	lastTransmitBatch = time.Now().Unix()
	messageQueue = append(messageQueue, msg)
	queueMutex.Unlock()
}

func resetModem() {
	pin, err := gpio.OpenPin(29, gpio.ModeOutput)
	defer pin.Close()
	if err != nil {
		panic(err)
	}
	pin.Set()
	time.Sleep(100 * time.Millisecond)
	pin.Clear()
	time.Sleep(100 * time.Millisecond)
}
func main() {
	resetModem()
	var (
		err error
		rfm *rfm69.Device
	)
	if conf, err = settings.LoadSettings(); err != nil {
		fmt.Errorf("Please check your .env File or Environment Vars")
		return
	}
	/*** RABBITMQ ***/
	rabbitmq.Debug = true
    connection, err := rabbitmq.Dial(conf.AMQPURL)
    if err != nil {
        panic("could not establish connection with RabbitMQ:" + err.Error())
    }
	publishCh, err := connection.Channel()
	if err != nil {
		log.Panic(err)
	}
	// Only if we allow TX, we process Messages
	if conf.TXFreq != 0 {
		consumeCh, err := connection.Channel()
		if err != nil {
			log.Panic(err)
		}
		if _, err := consumeCh.QueueDeclare("tx_pocsag", true, false, false, false, nil); err != nil {
			log.Panic(err)
		}
		go func() {
			d, err := consumeCh.Consume("tx_pocsag", "", false, false, false, false, nil)
			if err != nil {
				log.Panic(err)
			}

			for msg := range d {
				if msg.Headers["ric"] == nil { continue }
				log.Printf("ric: %s", string(msg.Headers["ric"].(int64)))
				log.Printf("msg: %s", string(msg.Body))
				queueMessage(&pocsagencode.Message {
					Addr: uint32(msg.Headers["ric"].(int64)),
					Content: string(msg.Body),
					IsNumeric: (msg.Headers["numeric"] != nil),
				})
				msg.Ack(true)
			}
		}()
	}
    /***** RABBITMQ END**/

	if rfm, err = rfm69.NewDevice(true); err != nil {
		log.Fatal(err)
	}

	rfm.FreqOffset = conf.FreqOffset

	rfm.TXFreq = conf.TXFreq
	rfm.TXBaud = conf.TXBaud

	rfm.RXFreq = conf.RXFreq
	rfm.RXBaud = conf.RXBaud

	if err = rfm.SetModeAndWait(rfm69.RF_OPMODE_STANDBY); err != nil {
		panic(err)
	}
	if err = rfm.SetInvert(conf.InvertBits); err != nil {
		panic(err)
	}

	log.Println("Running with following Config:")
	log.Println("AMQP Server: ", conf.AMQPURL)
	log.Println("Frequency Offset(Correction): ", conf.FreqOffset, "Hz")
	log.Println("Transmit Freq: ", conf.TXFreq, "Hz @", conf.TXBaud, "bps")
	log.Println("Receive Freq: ", conf.RXFreq, "Hz @", conf.RXBaud, "bps")

	if conf.RXFreq != 0 {
		err = publishCh.ExchangeDeclare("rx_pocsag", amqp.ExchangeFanout, true, false, false, false, nil)
		if err != nil {
			log.Panic(err)
		}
		rfm.OnReceive = func(stream *rfm69.RXStream) {
			rssiMeasurementArray := make([]int, 5)
			rssiStart := -0
			for {
				select {
				case rssi := <-stream.RSSI:
					//fmt.Printf("RSSI:%d\n", rssiStart - rssi)
					if (stream.ByteCounter < 20) {
						rssiMeasurementArray[int(stream.ByteCounter / 4)] = rssi
						rssiStart = 0
						for i := 0; i<5; i++ {
							rssiStart += rssiMeasurementArray[i]
						}
						rssiStart = rssiStart / 5
					} else {
						if rssiStart - rssi > 30 {
							stream.Cancel = true
						}
						if stream.ByteCounter >	1024e2 {
							stream.Cancel = true
						}
					}
					break
				case <-stream.Process:
					data := make([]byte, len(stream.ByteStream))
					//log.Println("--PROCESS=", len(stream.ByteStream), " bytes--")
					for i:= 0; i < len(stream.ByteStream); i++ {
						data[i] = <-stream.ByteStream
						//fmt.Printf("%x", data[i])
					}
					err := publishCh.Publish("rx_pocsag", "", false, false, amqp.Publishing{
						ContentType: "application/octet-stream",
						Body: data,
						Headers: map[string]interface{} {
							"rssi": rssiStart,
							"len": len(data),
						},
					})
					if err != nil {
						log.Panic(err)
					}
					//fmt.Print("\n")
					log.Println("--RX SENT--")
					break
				}
			}
		}
		rfm.PrepareRX()
	}

	// Only if we allow TX, we process Messages
	if conf.TXFreq != 0 {
		go func() {
			lastTransmitBatch = time.Now().Unix()
			for {
				log.Println(time.Now().Unix(), lastTransmitBatch + 5)

				time.Sleep(500 * time.Millisecond)
				if len(messageQueue) > 0 && (time.Now().Unix() > lastTransmitBatch + 5) {
					lastTransmitBatch = time.Now().Unix()
					log.Println("transmitting pocsag batch")
					queueMutex.Lock()
					burst, _ := pocsagencode.Generate(messageQueue, pocsagencode.OptionMaxLen(6000))
					log.Println("Transmitting", len(burst.Bytes()), "bytes")
					data := &rfm69.Data{
						Data: burst.Bytes(),
					}
					rfm.Send(data)
					messageQueue = make([]*pocsagencode.Message, 0)
					queueMutex.Unlock()
					// Make Pocsag Burst
				}
			}
		}()
	}
	rfm.Loop()
	log.Println("Done")
}
