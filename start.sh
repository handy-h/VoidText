#!/bin/bash
cd "$(dirname "$0")"
nohup ./voidtext > /dev/null 2>&1 &
echo "PID: $!"
