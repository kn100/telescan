# 🖨️ Telescan

Telescan is a Telegram Bot that allows you to use your Airscan compatible 
scanner from a conversation on Telegram. It will save your scans to a directory
of your choosing, and optionally send them to you through Telegram.

![](https://github.com/kn100/telescan/raw/master/demo.gif)

## Why?

There are projects out there designed for document indexing and archiving. One
such project is [Paperless-ngx](https://github.com/paperless-ngx/paperless-ngx)
it is a great project and I use it myself. The way it's supposed to be used is
you dump your scans into an import directory, and then Paperless-ngx will tag
and index them for you. I wanted a way to remove the use of a computer from the
process. Now, my process is to scan a document using Telescan, and it will
dump it into my Paperless-ngx import directory. Paperless-ngx will then do its
thing and I can search for the document later.

## Setup

1. Create a Telegram Bot using the [BotFather](https://telegram.me/BotFather).
2. Make sure your Telegram account has a username set.
3. Use the example Docker Compose file below to get started.
4. Start a chat with your new Telegram bot.

## Docker Compose

```yaml
version: '3.8'
services:
  telescan:
    build: .
    image: ghcr.io/kn100/telescan:latest
    # Host networking is essential since we need to be able to receive Bonjour 
    # broadcasts from the scanner. If you do not use host networking, you will
    # need to set up a Bonjour reflector on your network.
    network_mode: "host"
    container_name: telescan
    environment:
      # Get your API key from the BotFather on Telegram.
      TELEGRAM_API_KEY: your-api-key
      # Get your Telegram username from your Telegram profile. You may add more
      # than one username, separated by commas, if you wish to allow multiple
      # users to use your scanner. 
      AUTHORIZED_USERS: your-telegram-username,another-telegram-username,etc
      # Set to false to deny yourself the ability to ever receive scans through
      # Telegram. True if you want to be asked on each scan whether you want to
      # receive it or not. Note that Telegram is NOT a secure messaging platform
      # and you should not use it to send sensitive documents, if you do not 
      # trust Telegram.
      SEND_SCAN_TO_CHAT: true
      # Scanner name to insist on using. If you do not specify this variable,
      # the first scanner found will be used. There is usually no need to set 
      # this variable, unless you have multiple scanners and you want to use a
      # specific one.
      # SCANNER_OVERRIDE: your-scanner
    volumes:
      - "/tmp:/tmp"
      - "/final:/final"
```

Telescan will use the Automatic Document Feed capability of your scanner, if it supports it. It is not currently possible to override this behaviour. If your scanner does not feature this capability (ie, it is flatbed only), it'll use that.

## Running without Docker
```bash
TELEGRAM_API_KEY="<some-key>" \
AUTHORIZED_USERS="some-user" \
TMP_DIR="/tmp" \
FINAL_DIR="/somewhere" \
SEND_SCAN_TO_CHAT="false" \
go run main.go
```

# Device support
Tested this project on your scanner? Please either submit an issue letting us
know that it worked or didn't work, or edit the README.md to add it to the 
lists below.

## Working
* HP Envy 5010 (tested with v0.1.8)
* Epson WF-7830 (tested with v0.1.8)
* Epson WF-3720 (tested with v0.1.9)
* Brother MFC-L2710DN (tested with v0.1.8)

## Not working

## TODO
* The ADF simplex setup does not conveniently support scanning backsides. It is possible to insert the documents the other way and continue scanning, however this will result in the pages not being in the correct order. Add support for this interlacing if the user chooses to scan backsides.
* It would probably be nice to ask the user if they want to scan in A4 or Letter - rather than just forcing the superior standard (A4). I live in Canada so this default makes me suffer too.
* The separation between the scanner and the scan session complicates situations where the feature set of the scanner affects how the user scans. I am thinking specifically of the ADF Simplex setup issue described above. In future, after the scanner has concluded scanning all the front sides, Telescan should ask the user if they wish to scan backsides - but should only do this in the ADF Simplex case. This is going to be ugly unless we more tightly couple the scanning hardware to the scan session.