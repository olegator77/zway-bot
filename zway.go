package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	maxDeviceLevel  = 99
	minDeviceLevel  = 0
	stepDeviceLevel = 10
)

type ZWayDeviceLevel float64

func (zwl *ZWayDeviceLevel) UnmarshalJSON(b []byte) (err error) {
	str := string(b)
	switch str {
	case "\"on\"":
		*zwl = maxDeviceLevel
	case "", "\"off\"", "null","\"\"":
		*zwl = minDeviceLevel
	default:
		val, e := strconv.ParseFloat(str, 64)
		err = e
		*zwl = ZWayDeviceLevel(val)
	}

	return err
}

type ZWayDevice struct {
	DeviceType string `json:"deviceType"`
	H          int    `json:"h"`
	ID         string `json:"id"`
	Location   int    `json:"location"`
	Metrics    struct {
		Title string `json:"title"`
		Color struct {
			R int `json:"r"`
			G int `json:"g"`
			B int `json:"b"`
		} `json:"color"`
		Level     ZWayDeviceLevel `json:"level"`
		RgbColors string          `json:"rgbColors"`
	} `json:"metrics"`
	PermanentlyHidden bool          `json:"permanently_hidden"`
	Tags              []interface{} `json:"tags"`
	Visibility        bool          `json:"visibility"`
	UpdateTime        int           `json:"updateTime"`
}

type ZWayDevicesResp struct {
	Data struct {
		StructureChanged bool         `json:"structureChanged"`
		UpdateTime       int          `json:"updateTime"`
		Devices          []ZWayDevice `json:"devices"`
	} `json:"data"`
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Error   interface{} `json:"error"`
}

type ZWayAuthResp struct {
	Data struct {
		Sid string
	}
}

type ZWayLocation struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type ZWayLocationsResp struct {
	Data    []ZWayLocation `json:"data"`
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Error   interface{}    `json:"error"`
}

type ZWay struct {
	baseURL   string
	zwaySess  string
	devices   map[string]ZWayDevice
	locations map[int]ZWayLocation
	lock      sync.Mutex
}

func NewZWay(baseURL string) *ZWay {
	return &ZWay{baseURL: baseURL, devices: make(map[string]ZWayDevice), locations: make(map[int]ZWayLocation)}
}

func (zw *ZWay) Auth(name string, pass string) error {
	authData, _ := json.Marshal(map[string]interface{}{"login": name, "password": pass})
	req, _ := http.NewRequest("POST", zw.baseURL+"/login", bytes.NewBuffer(authData))

	authResp := ZWayAuthResp{}

	err := zw.request(req, &authResp)

	if len(authResp.Data.Sid) == 0 {
		err = fmt.Errorf("No token in answer")
	}

	if err == nil {
		log.Printf("Got ZWAYAuth token: %s", authResp.Data.Sid)
		zw.zwaySess = authResp.Data.Sid
	}
	return err
}

func (zw *ZWay) StartPolling(t time.Duration) {
	go func() {
		for {
			time.Sleep(t)
			zw.Devices(true)
		}
	}()
}

func (zw *ZWay) ControlRGB(dev string, r int, g int, b int) error {
	req, _ := http.NewRequest("GET", zw.baseURL+"/devices/"+dev+
		"/command/exact?red="+strconv.Itoa(r)+"&green="+strconv.Itoa(g)+"&blue="+strconv.Itoa(b), nil)

	return zw.request(req, nil)
}

func (zw *ZWay) ControlDimmer(dev string, level int) error {
	zw.saveDeviceLevel(dev, level)
	req, _ := http.NewRequest("GET", zw.baseURL+"/devices/"+dev+"/command/exact?level="+strconv.Itoa(level), nil)
	return zw.request(req, nil)
}

func (zw *ZWay) ControlOn(dev string) error {
	zw.saveDeviceLevel(dev, maxDeviceLevel)
	req, _ := http.NewRequest("GET", zw.baseURL+"/devices/"+dev+"/command/on", nil)
	return zw.request(req, nil)
}

func (zw *ZWay) ControlToggle(dev string) error {
	if zw.isDeviceOn(dev) {
		return zw.ControlOff(dev)
	}
	return zw.ControlOn(dev)

}
func (zw *ZWay) ControlDimmerUp(dev string) error {
	return zw.ControlDimmer(dev, zw.adjustDimmerVal(dev, stepDeviceLevel))
}

func (zw *ZWay) ControlDimmerDown(dev string) error {
	return zw.ControlDimmer(dev, zw.adjustDimmerVal(dev, -stepDeviceLevel))
}

func (zw *ZWay) ControlDimmerMax(dev string) error {
	return zw.ControlDimmer(dev, maxDeviceLevel)
}

func (zw *ZWay) ControlOff(dev string) error {
	zw.saveDeviceLevel(dev, 0)
	req, _ := http.NewRequest("GET", zw.baseURL+"/devices/"+dev+"/command/off", nil)
	return zw.request(req, nil)
}

func (zw *ZWay) Devices(forceReload bool) (ret []ZWayDevice, err error) {

	devices := ZWayDevicesResp{}
	if len(zw.devices) == 0 || forceReload {
		req, _ := http.NewRequest("GET", zwayURL+"/devices", nil)
		err := zw.request(req, &devices)
		if err != nil {
			return nil, err
		}
	}

	zw.lock.Lock()
	for _, d := range devices.Data.Devices {
		if d.Visibility && !d.PermanentlyHidden &&
			(d.DeviceType == "switchRGBW" ||
				d.DeviceType == "switchMultilevel" ||
				d.DeviceType == "toggleButton" ||
				d.DeviceType == "switchBinary" ||
				d.DeviceType == "thermostat") {
			zw.devices[d.ID] = d
		}
	}
	for _, d := range zw.devices {
		ret = append(ret, d)
	}
	zw.lock.Unlock()

	return ret, nil
}

func (zw *ZWay) Locations(forceReload bool) (ret []ZWayLocation, err error) {

	locations := ZWayLocationsResp{}
	if len(zw.locations) == 0 || forceReload {
		req, _ := http.NewRequest("GET", zwayURL+"/locations", nil)
		if err := zw.request(req, &locations); err != nil {
			return nil, err
		}
	}

	zw.lock.Lock()
	for _, loc := range locations.Data {
		zw.locations[loc.ID] = loc
	}
	for _, loc := range zw.locations {
		ret = append(ret, loc)
	}

	zw.lock.Unlock()
	return ret, nil
}

func (zw *ZWay) LocationTitle(id int) string {
	zw.lock.Lock()
	defer zw.lock.Unlock()
	return zw.locations[id].Title
}

func (zw *ZWay) DeviceTitle(id string) string {
	zw.lock.Lock()
	defer zw.lock.Unlock()
	return zw.devices[id].Metrics.Title
}

func (zw *ZWay) isDeviceOn(dev string) bool {
	zw.lock.Lock()
	d := zw.devices[dev]
	zw.lock.Unlock()
	return (d.DeviceType != "toggleButton" && d.Metrics.Level != 0)
}

func (zw *ZWay) adjustDimmerVal(dev string, adjust int) int {
	zw.lock.Lock()
	d := zw.devices[dev]
	zw.lock.Unlock()
	iVal := int(d.Metrics.Level) + adjust
	if iVal < minDeviceLevel {
		iVal = minDeviceLevel
	}
	if iVal > maxDeviceLevel {
		iVal = maxDeviceLevel
	}
	return iVal
}

func (zw *ZWay) saveDeviceLevel(dev string, level int) {
	zw.lock.Lock()
	d := zw.devices[dev]
	d.Metrics.Level = ZWayDeviceLevel(level)
	zw.devices[dev] = d
	zw.lock.Unlock()
}

func (zw *ZWay) request(req *http.Request, dest interface{}) error {

	req.Header.Add("ZWAYSession", zw.zwaySess)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return err
	}
	defer resp.Body.Close()
	log.Printf("ZWAYRequest: '%s' -> %d", req.URL.String(), resp.StatusCode)
	if dest == nil {
		return nil
	}
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(&dest)
}
