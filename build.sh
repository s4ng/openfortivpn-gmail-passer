#!/bin/bash
env GOOS=darwin GOARCH=amd64 go build -o gvpn
sudo codesign --verbose --timestamp -s "Apple Development: leesg951201@gmail.com (7364WSK56S)" gvpn
