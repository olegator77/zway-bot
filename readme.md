# zway-bot

The ZWay text bot is small tool to control ZWay server via human text commands. It can act as telegram bot and as http server.

## Installation

```
go get -u github.com/olegator77/zway-bot
```

## Usage

### Start bot

```
zway-bot -zway-url=<zway API server url> -zway-user=<zway user name> -zway-password=<zway password> -tg-bot-token='<telegram bot token' --tg-bot-users=<comma separated list of authorized telegram users>

```

### Using bot

Send text commands to bot like:

- 'turn on lamp in the hall'
- 'red illumination in the cabinet'
- 'green illumination'
- 'turn off TV in the bedroom'
- 'heat floor in the bathroom'

It parses text and matches it with commands, devices and locations titles.

