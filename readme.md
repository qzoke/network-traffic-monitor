## version: 
1.0

## Description:
Network Traffic Monitor is a program built to show current network activity as well as past network usage.


## Platforms:
It is currently only supported on Windows. Mac and linux
will be supported in later versions.

Supported Operating System : Windows
Tested only on Windows.

## build step:

Steps to create network monitor : 

1. Run "go mod tidy"
2. Run "go build -o NetworkTrafficMonitor.exe"
3. Open NetworkTrafficMonitor.exe
4. Open http://localhost:8080/ in your browser.

## screenshot:

![alt text](image.png)



## TODO
1. Graphs for upload and Download
2. Monthly , daily, hourly data tables. 
3. Accurate network usage data.
4. Program should Always run in background. 