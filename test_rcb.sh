#!/bin/bash
echo "Testing RCB attribute access..."
./iec61850ctl --host 192.168.100.57 --port 102 list das --ld MNSREF615LD0 --ln LLN0 --do rcbMeasFlt01
