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

type Schedule struct {
	Workday []struct {
		From        string  `json:"from"`
		To          string  `json:"to"`
		Temperature float64 `json:"temperature"`
	} `json:"workday"`
	Freeday []struct {
		From        string  `json:"from"`
		To          string  `json:"to"`
		Temperature float64 `json:"temperature"`
	} `json:"freeday"`
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

func onMessage(client mqtt.Client, message mqtt.Message) {
	// unmarshal message into old schedule typr
	old := OldSchedule{}
	err := json.Unmarshal(message.Payload(), &old)
	if err != nil {
		log.Println(err)
		return
	}
	// convert schedule
	new := convert(old)

	// marshal schedule into string
	s, err := json.Marshal(new)
	if err != nil {
		log.Println(err)
		return
	}

	// send new schedule
	log.Panicln(s)
	//client.Publish(scheduleTopic, 0, true, s)

	// set expected temperature (for manual override)
	setExpected(old.Override.Temp)

}

func convert(old OldSchedule) Schedule {
	s := Schedule{}
	s.DefaultTemperature = old.Other
	for i, entry := range old.Free {
		s.Freeday[i].From = parseHour(entry.From)
		s.Freeday[i].To = parseHour(entry.To)
		s.Freeday[i].Temperature = 0
	}
	for i, entry := range old.Work {
		s.Workday[i].From = parseHour(entry.From)
		s.Workday[i].To = parseHour(entry.To)
		s.Workday[i].Temperature = 0
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
