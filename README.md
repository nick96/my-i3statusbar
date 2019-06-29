# Custom i3 status bar

This is the custom i3 status bar that I use, build using [barista](https://github.com/soumya92/barista). 
It provides:

  - Basic lastpass integration (through `lpass` CLI)
  - Info about disk usage
  - WiFi connection info (SSID and strength)
  - Ethernet connection info
  - Battery info
  - Date and time

It this based off the [sample bar](https://github.com/soumya92/barista/blob/master/samples/sample-bar/sample-bar.go)
provided by barista

## Build and install

``` bash
go install
```

This will install the status bar in `$GOPATH` as `my-i3statusbar`. You can then use it in your i3 config:


```
bar {
    status_command exec my-i3statusbar -email "<email>"
}
```

`email` is the email you wish to use for your LastPass account. If this is not provided, no LastPass segment
will appear in the bar.

## Features

Apart from the visual information provided in the bar. When the "LastPass" segment is red, you can click
on it to log back into the LastPass CLI.
