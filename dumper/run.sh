#!/bin/bash

for pid in $(pgrep WeChat); do
    echo $pid
    kill -9 $pid
done

open -a WeChat

sleep 1

lldb -p $(pgrep WeChat) -s mem_dumper.lldb
cat .memory.hexdump | awk '{for(i=2;i<=9;i++) printf("%s", substr($i, 3))}' | tee /dev/tty | pbcopy
