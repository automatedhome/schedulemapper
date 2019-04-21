package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"strconv"

	mqttclient "github.com/automatedhome/flow-meter/pkg/mqttclient"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Day struct {
	From        string  `json:"from"`
	To          string  `json:"to"`
	Temperature float64 `json:"temperature"`
}

type Schedule struct {
	Workday            []Day   `json:"workday"`
	Freeday            []Day   `json:"freeday"`
	DefaultTemperature float64 `json:"defaultTemperature"`
}

type OldSchedule struct {
	Week     []int `json:"week"`
	Override struct {
		Duration int     `json:"duration"`
		Start    []int   `json:"start"`
		Temp     float64 `json:"temp"`
	} `json:"override"`
	Work []struct {
		To   []int   `json:"to"`
		From []int   `json:"from"`
		Temp float64 `json:"temp"`
	} `json:"work"`
	Other float64 `json:"other"`
	Free  []struct {
		To   []int   `json:"to"`
		From []int   `json:"from"`
		Temp float64 `json:"temp"`
	} `json:"free"`
}

var (
	oldScheduleTopic string
	scheduleTopic    string
	overrideTopic    string
)
var expected float64
var schedule Schedule
var client mqtt.Client

func (s *Schedule) addWorkday(item Day) []Day {
	s.Workday = append(s.Workday, item)
	return s.Workday
}

func (s *Schedule) addFreeday(item Day) []Day {
	s.Freeday = append(s.Freeday, item)
	return s.Freeday
}

func onMessage(client mqtt.Client, message mqtt.Message) {
	// unmarshal message into old schedule typr
	old := OldSchedule{}
	payload := message.Payload()[1:]
	payload = payload[0 : len(payload)-1]
	err := json.Unmarshal(payload, &old)
	if err != nil {
		log.Printf("Cannot unmarshal old schedule: %v\n", err)
		return
	}
	// convert schedule
	new := convert(old)

	// marshal schedule into string
	s, err := json.Marshal(new)
	if err != nil {
		log.Printf("Cannot marshal schedule into string: %v\n", err)
		return
	}

	// send new schedule
	client.Publish(scheduleTopic, 0, true, s)

	// set expected temperature (for manual override)
	setExpected(old.Override.Temp)

}

func convert(old OldSchedule) Schedule {
	s := Schedule{}
	s.DefaultTemperature = old.Other
	new := Day{}
	for _, entry := range old.Free {
		new.From = parseHour(entry.From)
		new.To = parseHour(entry.To)
		new.Temperature = entry.Temp
		s.addFreeday(new)
	}
	for _, entry := range old.Work {
		new.From = parseHour(entry.From)
		new.To = parseHour(entry.To)
		new.Temperature = entry.Temp
		s.addWorkday(new)
	}

	log.Printf("Parsed schedule: %+v", s)
	return s
}

func parseHour(hour []int) string {
	h := strconv.Itoa(hour[0])
	m := strconv.Itoa(hour[1])
	return h + ":" + m
}

func setExpected(value float64) {
	if expected == 0 {
		log.Printf("Initializing internal expected temperature value to %.2f and performing no action", value)
		expected = value
		return
	}
	if value != expected {
		client.Publish(overrideTopic, 0, false, fmt.Sprintf("%.2f", value))
		expected = value
		log.Printf("Setting expected temperature to %.2f", value)
	}
}

func init() {
	oldScheduleTopic = "solarControl/heater/settings/schedule"
	overrideTopic = "unipi/heater/settings/override"
	scheduleTopic = "unipi/heater/settings/schedule"
	expected = 0
}

func main() {
	broker := flag.String("broker", "tcp://192.168.20.3:1883", "The full url of the MQTT server to connect to.")
	clientID := flag.String("clientid", "schedulemapper", "A clientid for the connection")
	flag.Parse()

	brokerURL, _ := url.Parse(*broker)

	var topics []string
	topics = append(topics, oldScheduleTopic)
	client = mqttclient.New(*clientID, brokerURL, topics, onMessage)
	log.Printf("Connected to %s as %s and waiting for messages\n", *broker, *clientID)

	// wait forever
	select {}
}
