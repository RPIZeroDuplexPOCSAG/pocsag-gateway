# pocsag-gateway

This WIP Software aims to provide an Interface for transmitting Messages sent over AMQP via an RFM69 Transceiver.
## Software Configuration
Configuration works via Environment Variables or a `.env` file.
```
POCGW_AMQPURL=amqp://guest:guest@10.13.37.37:5672

POCGW_TXBAUD=1200
POCGW_RXBAUD=1200

POCGW_TXFREQ=434230000
POCGW_RXFREQ=434230000

POCGW_FREQOFFSET=25
```
If TX or RX Frequency are not set or set to 0, the software will not use this mode(only-rx or only-tx). If both are set, then rx is always active until there are pages to tx.

## Pin Configuration
| RFM pin | Pi pin  
| ------- |-------
| DIO0    | 18 (GPIO24)  
| MOSI    | 19  
| MISO    | 21  
| CLK     | 23  
| NSS     | 24  
| Ground  | 25  
| RESET   | 29
