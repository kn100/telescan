# telescan

Telescan is a Telegram Bot that allows you to use your Airscan compatible 
scanner from a conversation on Telegram.

## But why?

There are projects out there designed for document indexing and archiving. One
such project is [Paperless-ngx](https://github.com/paperless-ngx/paperless-ngx)
it is a great project and I use it myself. The way it's supposed to be used is
you dump your scans into an import directory, and then Paperless-ngx will tag
and index them for you. I wanted a way to remove the use of a computer from the
process. Now, my process is to scan a document using Telescan, and it will
dump it into my Paperless-ngx import directory. Paperless-ngx will then do its
thing and I can search for the document later.

## How do I use it?

1. Create a Telegram Bot using the [BotFather](https://telegram.me/BotFather).
2. Make sure your Telegram account has a username set.
2. Use the example Docker Compose file below to get started.
3. Start a chat with your new Telegram bot.

## Docker Compose
Replace the environment variables with your own values. The `AUTHORIZED_USERS`
variable is a comma separated list of Telegram usernames that are allowed to
use the bot. The `SCANNER_OVERRIDE` variable is optional and can be used to
override the scanner name that is used. If you don't specify this variable,
the first scanner found will be used. Then, update the volumes to match your
setup. The `/final` directory is where the final PDF will be placed. The
`/tmp` directory is where the individual pages will be placed before being
combined into a PDF. The `/etc/localtime` volume is optional and is used to
set the timezone of the container to match the host. Host networking is essential
since we need to be able to receive Bonjour broadcasts from the scanner.

```yaml
version: '3.8'
services:
  telescan:
    build: .
    image: kn100/telescan:latest
    network_mode: "host"
    container_name: telescan
    environment:
      - TELEGRAM_API_KEY=your-api-key
      - AUTHORIZED_USERS=your-telegram-username,another-telegram-username,etc
      # SCANNER_OVERRIDE="your-scanner"
    volumes:
      - "/etc/localtime:/etc/localtime:ro"
      - "/tmp:/tmp"
      - "/final:/final"
```

## What does it look like?

![](https://github.com/kn100/telescan/raw/master/demo.gif)