# zway-bot

The ZWay text bot is small tool to control ZWay server via human text commands. It can act as telegram bot and as http server.

## Installation

```
go get -u github.com/olegator77/zway-bot
```

## Usage

### Start bot

```
zway-bot -zway-url=<zway API server url> \
    -zway-user=<zway user name> \
    -zway-password=<zway password> \
    -tg-bot-token='<telegram bot token' \
    -tg-bot-users=<comma separated list of authorized telegram users> \
    -http-addr=<http server addr:port> \
    -bind-locations=<Comma separated bindings of sender's default locations, e.g 'olegator77=cabinet,192.168.1.101=hall'>

```

### Using bot

Just send text phrases to the bot like:
- `turn on lamp in the lounge`
- `red illumination in the cabinet`
- `green illumination`
- `turn off TV in the bedroom`
- `heat floor in the bathroom and toilet`

Bot will parse phrase, match it with commands, devices and locations titles obtained from ZWay server.

Supported comamnds are:
- `on` - turn on the device
- `off` - turn off the device
- `run` - run scene
- `red/green/yellow/white...` - set color to RGB lamp illumination
- `maximum` - set maximum level to dimmer
- `lighter` - increase dimmer level
- `darker` - decrease dimmer level

### Control contexts

Bot is remember last devices and locations, and uses them for next commands to last devices or last location. Contexts are binded to commands's sender: telegram nick or IP address of remote host.

E.g: after command `turn on dimmer in the cabinet`, the next command can be in short form, like just `off` or `maximum`. The device `dimmer` and location `cabinet` will be used from last context.

### Default locations

Bot can use default location of command sender (telegram nick or IP address of remote host). This default location will be used, if command phrase is not contains location name.
