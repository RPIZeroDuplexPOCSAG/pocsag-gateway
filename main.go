package main

import (
	"fmt"
	"time"
	//"sync"
	"log"
	"github.com/kgolding/go-pocsagencode"
	"github.com/RPIZeroDuplexPOCSAG/rfm69"

	"github.com/davecheney/gpio"

	"github.com/sirius1024/go-amqp-reconnect/rabbitmq"



	"github.com/RPIZeroDuplexPOCSAG/go-pocsag/notint_ernal/datatypes"
	"github.com/RPIZeroDuplexPOCSAG/go-pocsag/notint_ernal/pocsag"
	//"github.com/RPIZeroDuplexPOCSAG/go-pocsag/notint_ernal/utils"

	"github.com/RPIZeroDuplexPOCSAG/pocsag-gateway/settings"
)
var (
	conf  *settings.App
)
var messageQueue []*pocsagencode.Message 

func queueMessage(msg *pocsagencode.Message) {
	messageQueue = append(messageQueue,msg)
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
	consumeCh, err := connection.Channel()
	if err != nil {
		log.Panic(err)
	}
	// Only if we allow TX, we process Messages
	if conf.TXFreq != 0 {
		go func() {
			d, err := consumeCh.Consume("input", "", false, false, false, false, nil)
			if err != nil {
				log.Panic(err)
			}

			for msg := range d {
				if msg.Headers["ric"] == nil { continue }
				log.Printf("ric: %s", string(msg.Headers["ric"].(string)))
				log.Printf("msg: %s", string(msg.Body))
				/*queueMessage(&pocsagencode.Message {
					Addr: uint32(msg.Headers["ric"].(int)),
					Content: string(msg.Body),
					IsNumeric: (msg.Headers["numeric"] != nil),
				})*/
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
/*
	messages := []*pocsagencode.Message{
		&pocsagencode.Message{133701, "Hello 1234567890!", false},
		//&pocsagencode.Message{133702, "Hello d2efa947-7618-440c-8f79-fab32762af8ed2bb9c62-007e-4b2c-93d5-3124a247032eefe71db4-ef8d-46fb-9cf8-dac70db000bc12067966-da61-447c-a9ce-c0c24be17df5 Pager!", false},
		//&pocsagencode.Message{133703, "Hello c41554ca-7372-4975-b0c3-3e2eaccc1e8b Pager!", false},
		//&pocsagencode.Message{133704, "Hello Pager!", false},
	}

	log.Println("Sending", len(messages), "messages")
	
	var burst pocsagencode.Burst
	for len(messages) > 220 {
		burst, messages = pocsagencode.Generate(messages, pocsagencode.OptionMaxLen(6000))
		// Options can be set as below for MaxLen and PreambleBits
		// burst, messages = pocsagencode.Generate(messages, pocsagencode.OptionPreambleBits(250))
		//log.Println("% X\n\n", burst.Bytes())
		log.Println(len(burst.Bytes()))
		data := &rfm69.Data{
			Data: burst.Bytes(),
		}
		rfm.Send(data)
	}
*/
	if conf.RXFreq != 0 {
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
				/*case byte := <-stream.ByteStream:
					fmt.Printf("%x", byte)
					break*/
				case <-stream.Process:
					for i := 0; i < 5; i++ {
						<-stream.ByteStream
					}
					/*for i := 0; i < 32; i++ {
						bits = append(bits, false)
					}*/
					var tByte = 0
					var bitstring = ""
					log.Println("--PROCESS=", len(stream.ByteStream), " bytes--")
					for i:= 0; i < len(stream.ByteStream); i++ {
						tByte = int(<-stream.ByteStream)
						if tByte == 0xAA && bitstring == "" {
							continue
						}
						fmt.Printf("%x", tByte)
						for BB := 7; BB > -1; BB-- {
							// Compare bits 7-0 in byte
							if tByte & (1 << uint(BB)) > 0 {
								bitstring = bitstring + "1"
							} else {
								bitstring = bitstring + "0"
							}
						}
						// bitstring = bitstring + fmt.Sprintf("%b", tByte)
						/*bits = append(bits, datatypes.Bit(tByte & 128 > 0))
						bits = append(bits, datatypes.Bit(tByte & 64 > 0))
						bits = append(bits, datatypes.Bit(tByte & 32 > 0))
						bits = append(bits, datatypes.Bit(tByte & 16 > 0))
						bits = append(bits, datatypes.Bit(tByte & 8 > 0))
						bits = append(bits, datatypes.Bit(tByte & 4 > 0))
						bits = append(bits, datatypes.Bit(tByte & 2 > 0))
						bits = append(bits, datatypes.Bit(tByte & 1 > 0))*/
					}
					// bitstring = fmt.Sprintf("%b", 0xAA)+ fmt.Sprintf("%b", 0xAA)+ fmt.Sprintf("%b", 0xAA)+ fmt.Sprintf("%b", 0xAA) + bitstring
					fmt.Print("\n")
					//fmt.Print(bitstring)
					fmt.Print("\n")
					for i := 0; i < 512; i++ {
						bitstring = bitstring + "0"
					}
					var bits = make([]datatypes.Bit, len(bitstring))
					for i, c := range bitstring {
						if string(c) == "1" {
							bits[i] = datatypes.Bit(true)
						} else {
							bits[i] = datatypes.Bit(false)
						}
					}
					parsedMsgs := pocsag.ParsePOCSAG(bits, pocsag.MessageTypeAuto)
					for _, m := range parsedMsgs {
						log.Println(m.ReciptientString())
						log.Println(m.PayloadString(pocsag.MessageTypeAuto))
					}
					log.Println("--END--")
					break
				}
			}
		}
		rfm.PrepareRX()
	}

	rfm.Loop()
	log.Println("Done")
}
