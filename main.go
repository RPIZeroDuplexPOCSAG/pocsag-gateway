package main

import (
	"os"
	"fmt"
	"time"
	"sync"
	"log"
	"github.com/kgolding/go-pocsagencode"
	"github.com/RPIZeroDuplexPOCSAG/rfm69"
	"github.com/davecheney/gpio"

    //"github.com/streadway/amqp"
	"github.com/sirius1024/go-amqp-reconnect/rabbitmq"
)

var messageQueue []*pocsagencode.Message 

func queueMessage(msg *pocsagencode.Message) {
	messageQueue = append(messageQueue,msg)
}
func main2() {
	rabbitmq.Debug = true
    url := os.Getenv("AMQP_URL")
    if url == "" {
        url = "amqp://guest:guest@10.13.37.37:5672"
	}
    connection, err := rabbitmq.Dial(url)
    if err != nil {
        panic("could not establish connection with RabbitMQ:" + err.Error())
    }

	consumeCh, err := connection.Channel()
	if err != nil {
		log.Panic(err)
	}
	go func() {
		d, err := consumeCh.Consume("input", "", false, false, false, false, nil)
		if err != nil {
			log.Panic(err)
		}

		for msg := range d {
			if msg.Headers["ric"] == nil { continue }
			log.Printf("ric: %s", string(msg.Headers["ric"].(string)))
			log.Printf("msg: %s", string(msg.Body))
			queueMessage(&pocsagencode.Message {
				Addr: uint32(msg.Headers["ric"].(int)),
				Content: string(msg.Body),
				IsNumeric: (msg.Headers["numeric"] != nil),
			})
			msg.Ack(true)
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)

	wg.Wait()
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
	rfm, err := rfm69.NewDevice(true)
	if err != nil {
		log.Fatal(err)
	}

	if err = rfm.SetModeAndWait(rfm69.RF_OPMODE_STANDBY); err != nil {
		log.Fatal(err)
	}
	//rfm.WriteReg(0x25, 0x00);
	////rfm.SetFrequency(446118750, 25)
	rfm.SetFrequency(434230000, 25)
	//rfm.SetFrequency(466238000, 25)
	rfm.SetInvert(false)
	if err = rfm.SetBaudrate(1200); err != nil {
		panic(err)
	}

	messages := []*pocsagencode.Message{
		&pocsagencode.Message{133701, "Hello 1234567890!", false},
		//&pocsagencode.Message{133702, "Hello d2efa947-7618-440c-8f79-fab32762af8ed2bb9c62-007e-4b2c-93d5-3124a247032eefe71db4-ef8d-46fb-9cf8-dac70db000bc12067966-da61-447c-a9ce-c0c24be17df5 Pager!", false},
		//&pocsagencode.Message{133703, "Hello c41554ca-7372-4975-b0c3-3e2eaccc1e8b Pager!", false},
		//&pocsagencode.Message{133704, "Hello Pager!", false},
	}

	//for i := 0; i < 1; i++ {
	//	addr := uint32(1200000 + i*100)
	//	messages = append(messages, &pocsagencode.Message{addr, fmt.Sprintf("Hello pager number %d", addr), false})
	//}

	log.Println("Sending", len(messages), "messages")
	
	var burst pocsagencode.Burst
	for len(messages) > 0 {
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
					if rssiStart - rssi > 20 {
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
				log.Println("--PROCESS=", len(stream.ByteStream), " bytes--")
				for i:= 0; i < len(stream.ByteStream); i++  {
					fmt.Printf("%x", <-stream.ByteStream)
				}
				fmt.Print("\n")
				log.Println("--END--")
				break
			}
		}
	}
	rfm.Loop()
	log.Println("Done")
}