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

	{"активируй", []string{"toggleButton"}, CommandOn, nil},
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
	lastCmdTime     time.Time
	lastCmdLocation int
	lastCmdDevice   string
	defaultLocation int
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

func (cmd *CmdProcessor) ProcessPhrase(phrase string, ctxName string) (devID string, locID int, cmdPtr *CommandDef) {

	ctx, found := cmd.contexts[ctxName]
	if !found {
		ctx = &context{}
		cmd.contexts[ctxName] = ctx
	}

	locID = cmd.lookupLocation(phrase, ctx)

	excludeDevs := make(map[string]bool)
	devScore := 0
	for {
		devID, devScore = cmd.lookupDevice(phrase, locID, excludeDevs, ctx)
		if devID == "" {
			return "", 0, nil
		}
		excludeDevs[devID] = true
		cmdPtr = cmd.lookupCommand(phrase, cmd.devices[devID].DevType)
		if cmdPtr != nil || cmd.lookupCommand(phrase, "*") == nil {
			break
		}
	}

	if devScore == 0 && cmdPtr == nil {
		return "", 0, nil
	}

	ctx.lastCmdDevice = devID
	ctx.lastCmdLocation = locID
	ctx.lastCmdTime = time.Now()

	return devID, locID, cmdPtr
}

func (cmd *CmdProcessor) lookupDevice(phrase string, location int, excludeDevs map[string]bool, ctx *context) (string, int) {

	bestDevScore, bestDevID := 0, ""
	for id, dev := range cmd.devices {
		if _, excluded := excludeDevs[id]; excluded {
			continue
		}

		score := getTitleScore(phrase, dev.Title)

		if location != 0 && dev.IDLocation != location {
			score--
		}

		if score > bestDevScore {
			bestDevScore = score
			bestDevID = id
		}
	}

	if _, excluded := excludeDevs[ctx.lastCmdDevice]; !excluded && bestDevScore == 0 && time.Now().Sub(ctx.lastCmdTime) < time.Duration(60*time.Second) {
		bestDevID = ctx.lastCmdDevice
	}

	if bestDevID == "" {
		log.Printf("Can't lookup device")
	} else if bestDevScore != 0 {
		log.Printf("Looked up device '%s', score=%d", cmd.devices[bestDevID].Title, bestDevScore)
	} else {
		log.Printf("Using device '%s' from last context", cmd.devices[bestDevID].Title)
	}

	return bestDevID, bestDevScore
}

func (cmd *CmdProcessor) lookupLocation(phrase string, ctx *context) int {

	bestLocScore, bestLocID := 0, 0

	for id, loc := range cmd.locations {

		score := getTitleScore(phrase, loc.Title)

		if score > bestLocScore && cmd != nil {
			bestLocScore = score
			bestLocID = id
		}
	}

	if bestLocScore == 0 {
		if time.Now().Sub(ctx.lastCmdTime) < time.Duration(60*time.Second) {
			bestLocID = ctx.lastCmdLocation
		} else {
			bestLocID = ctx.defaultLocation
		}
	}

	if bestLocID == 0 {
		log.Printf("Can't lookup location")
	} else if bestLocScore != 0 {
		log.Printf("Locked up location '%s', score=%d", cmd.locations[bestLocID].Title, bestLocScore)
	} else {
		log.Printf("Using location '%s' from last context", cmd.locations[bestLocID].Title)
	}

	return bestLocID
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
