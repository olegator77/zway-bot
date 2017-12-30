package main

import (
	"bytes"
	"log"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kljensen/snowball"
)

const (
	CommandOn = iota
	CommandOff
	CommandToggle
	CommandRGB
	CommandDimmerUp
	CommandDimmerDown
	CommandDimmerMax
)

type CommandDef struct {
	Words    string
	DevTypes []string
	Command  int
	CmdData  interface{}
}

type CommandDataRGB struct {
	R, G, B int
}

var commands = [...]CommandDef{
	{"run", []string{"toggleButton"}, CommandOn, nil},
	{"on", []string{"*"}, CommandOn, nil},
	{"off", []string{"*"}, CommandOff, nil},

	{"ligher", []string{"switchMultilevel"}, CommandDimmerUp, nil},
	{"darker", []string{"switchMultilevel"}, CommandDimmerDown, nil},
	{"maximum", []string{"switchMultilevel"}, CommandDimmerMax, nil},

	{"red", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{100, 0, 0}},
	{"dark blue", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{0, 0, 100}},
	{"green", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{0, 100, 0}},
	{"blue", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{0, 100, 100}},
	{"white", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{100, 100, 100}},
	{"yellow", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{100, 100, 0}},
	{"violet", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{100, 0, 100}},

	{"запуск", []string{"toggleButton"}, CommandOn, nil},
	{"включи", []string{"*"}, CommandOn, nil},
	{"зажги", []string{"*"}, CommandOn, nil},
	{"погаси", []string{"*"}, CommandOff, nil},
	{"отключи", []string{"*"}, CommandOff, nil},
	{"выключи", []string{"*"}, CommandOff, nil},

	{"ярче", []string{"switchMultilevel"}, CommandDimmerUp, nil},
	{"светлее", []string{"switchMultilevel"}, CommandDimmerUp, nil},
	{"темнее", []string{"switchMultilevel"}, CommandDimmerDown, nil},
	{"максимум", []string{"switchMultilevel"}, CommandDimmerMax, nil},
	{"больше", []string{"switchMultilevel"}, CommandDimmerUp, nil},
	{"меньше", []string{"switchMultilevel"}, CommandDimmerDown, nil},

	{"красный", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{100, 0, 0}},
	{"синий", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{0, 0, 100}},
	{"зеленый", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{0, 100, 0}},
	{"голубой", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{0, 100, 100}},
	{"белый", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{100, 100, 100}},
	{"желтый", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{100, 100, 0}},
	{"фиолетовый", []string{"switchRGBW"}, CommandRGB, CommandDataRGB{100, 0, 100}},
}

type CmdLocation struct {
	Title string
}
type CmdDevice struct {
	Title      string
	DevType    string
	IDLocation int
}

type context struct {
	lastCmdTime      time.Time
	lastCmdLocations []int
	lastCmdDevices   []string
	defaultLocation  int
}

func (ctx *context) isExpired() bool {
	return time.Now().Sub(ctx.lastCmdTime) > time.Duration(60*time.Second)
}

type CmdProcessor struct {
	locations map[int]CmdLocation
	devices   map[string]CmdDevice
	locNames  map[string]int

	contexts map[string]*context
}

func NewCmdProcessor() *CmdProcessor {
	return &CmdProcessor{
		locations: make(map[int]CmdLocation),
		devices:   make(map[string]CmdDevice),
		locNames:  make(map[string]int),
		contexts:  make(map[string]*context),
	}
}

func (cmd *CmdProcessor) AddDevice(id, title, devType string, location int) string {
	titles := splitPhrase(title)

	title = ""
	for _, t := range titles {
		if _, found := cmd.locNames[t]; !found {
			title = title + t + " "
		}
	}

	cmd.devices[id] = CmdDevice{title, devType, location}
	return title
}

func (cmd *CmdProcessor) AddLocation(id int, title string) string {
	if id == 0 {
		title = "везде"
	}

	title = strings.Join(splitPhrase(title), " ")
	cmd.locNames[title] = id

	cmd.locations[id] = CmdLocation{title}
	return title
}

func (cmd *CmdProcessor) SetContextDefaultLocation(ctxName string, defaultLocTitle string) bool {

	defaultLocTitle = strings.Join(splitPhrase(defaultLocTitle), " ")
	locID, found := cmd.locNames[defaultLocTitle]
	if !found {
		return false
	}
	cmd.contexts[ctxName] = &context{defaultLocation: locID}
	return true
}

func (cmd *CmdProcessor) ProcessPhrase(phrase string, ctxName string) (devIDs []string, locIDs []int, cmdPtr *CommandDef) {

	ctx, found := cmd.contexts[ctxName]
	if !found {
		ctx = &context{}
		cmd.contexts[ctxName] = ctx
	}

	locIDs = cmd.lookupLocation(phrase, ctx)

	excludeDevs := make(map[string]bool)
	devScore := 0
	for {
		devIDs, devScore = cmd.lookupDevice(phrase, locIDs, excludeDevs, ctx)
		if len(devIDs) == 0 {
			return nil, nil, nil
		}
		cmdPtr = cmd.lookupCommand(phrase, cmd.devices[devIDs[0]].DevType)
		if cmdPtr != nil || cmd.lookupCommand(phrase, "*") == nil {
			break
		}
		for _, devID := range devIDs {
			excludeDevs[devID] = true
		}
	}

	if devScore == 0 && cmdPtr == nil {
		return nil, nil, nil
	}

	ctx.lastCmdDevices = devIDs
	ctx.lastCmdLocations = locIDs
	ctx.lastCmdTime = time.Now()

	return devIDs, locIDs, cmdPtr
}

func (cmd *CmdProcessor) lookupDevice(phrase string, locations []int, excludeDevs map[string]bool, ctx *context) ([]string, int) {

	bestDevScore, bestDevIDs := 0, []string{}
	for id, dev := range cmd.devices {

		// skip device, if it was already exclude on previous iterations
		if _, excluded := excludeDevs[id]; excluded {
			continue
		}

		// get phrase score
		score := getTitleScore(phrase, dev.Title)

		// check if device doesn't match location decrease score
		if len(locations) > 0 && locations[0] != 0 {
			locMatch := false
			for _, loc := range locations {
				locMatch = locMatch || (dev.IDLocation == loc)
			}
			if !locMatch {
				score--
			}
		}

		if score > bestDevScore {
			// Found device with better score - reset previous result
			bestDevScore = score
			bestDevIDs = bestDevIDs[:0]
		}

		if score != 0 && score == bestDevScore {
			// Device has current maximum score - append to result
			bestDevIDs = append(bestDevIDs, id)
		}
	}

	if len(bestDevIDs) == 0 && !ctx.isExpired() {
		// No devices found. Try fallback to device from context
		for _, devID := range ctx.lastCmdDevices {
			if _, excluded := excludeDevs[devID]; !excluded {
				bestDevIDs = append(bestDevIDs, devID)
			}
		}
	}

	if len(bestDevIDs) == 0 {
		log.Printf("Can't lookup device")
	} else {
		devNames := ""
		for i, devID := range bestDevIDs {
			if i != 0 {
				devNames += ","
			}
			devNames += cmd.devices[devID].Title
		}
		if bestDevScore != 0 {
			log.Printf("Looked up '%s', score=%d", devNames, bestDevScore)
		} else {
			log.Printf("Using '%s' from last context", devNames)
		}
	}

	return bestDevIDs, bestDevScore
}

func (cmd *CmdProcessor) lookupLocation(phrase string, ctx *context) []int {

	bestLocScore, bestLocIDs := 0, []int{}

	for id, loc := range cmd.locations {

		score := getTitleScore(phrase, loc.Title)

		if score > bestLocScore {
			bestLocScore = score
			bestLocIDs = bestLocIDs[:0]
		}
		if score != 0 && score == bestLocScore {
			bestLocIDs = append(bestLocIDs, id)
		}
	}

	if bestLocScore == 0 {
		if !ctx.isExpired() {
			bestLocIDs = ctx.lastCmdLocations
		} else {
			bestLocIDs = []int{ctx.defaultLocation}
		}
	}

	if len(bestLocIDs) == 0 {
		log.Printf("Can't lookup location")
	} else {
		locNames := ""
		for i, locID := range bestLocIDs {
			if i != 0 {
				locNames += ","
			}
			locNames += cmd.locations[locID].Title
		}

		if bestLocScore != 0 {
			log.Printf("Locked up location '%s', score=%d", locNames, bestLocScore)
		} else {
			log.Printf("Using location '%s' from last context", locNames)
		}
	}
	return bestLocIDs
}

func (cmd *CmdProcessor) lookupCommand(command, devType string) *CommandDef {
	wordsCmd := splitPhrase(command)

	var cmdDef *CommandDef

	for _, w := range wordsCmd {
		for cmdID, c := range commands {

			cmdWord, _ := snowball.Stem(c.Words, "russian", true)

			if isDevTypeMatch(devType, c.DevTypes) && w == cmdWord {
				cmdDef = &commands[cmdID]
			}
		}
	}
	if cmdDef != nil {
		log.Printf("Locked up command '%s'", cmdDef.Words)
	} else if devType == "*" {
		log.Printf("Can't lookup command '%s'", command)
	}

	return cmdDef
}

func (cmd *CmdProcessor) GetLocationTitle(id int) string {
	return cmd.locations[id].Title
}

func isDevTypeMatch(devType string, types []string) bool {
	if (len(types) == 1 && types[0] == "*") || devType == "*" {
		return true
	}

	for _, t := range types {
		if t == devType {
			return true
		}
	}
	return false
}

func splitPhrase(phrase string) []string {
	buf := bytes.Buffer{}

	for _, r := range phrase {
		if unicode.IsLetter(r) || r == ' ' {
			if r == 'ё' {
				r = 'е'
			}
			buf.WriteRune(unicode.ToLower(r))
		} else {
			buf.WriteRune(' ')
		}
	}

	words := strings.Split(buf.String(), " ")
	rwords := make([]string, 0, len(words))
	for i := range words {
		word, err := snowball.Stem(words[i], "russian", true)
		if err == nil && utf8.RuneCountInString(word) > 1 {
			rwords = append(rwords, word)
		}
	}
	return rwords
}

func getTitleScore(phrase, title string) int {
	wordsPhrase := splitPhrase(phrase)
	wordsTitle := splitPhrase(title)
	score := 0

	for _, pw := range wordsPhrase {
		for _, tw := range wordsTitle {
			if tw == pw {
				score++
			}
		}
	}

	return score
}
