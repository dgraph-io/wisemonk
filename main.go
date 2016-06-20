/*
 * Copyright 2016 DGraph Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nlopes/slack"
)

var yoda []byte

const slackPrefix = "https://slack.com/api"

// Message to send when number of messages in an interval >= *maxmsg. We send
// the Go Proverbs so that we learn all of them eventually :P.
var proverbs []string = []string{
	"Don't communicate by sharing memory, share memory by communicating.",
	"Concurrency is not parallelism.",
	"Channels orchestrate; mutexes serialize.",
	"The bigger the interface, the weaker the abstraction.",
	"Make the zero value useful.",
	"interface{} says nothing.",
	"Gofmt's style is no one's favorite, yet gofmt is everyone's favorite.",
	"A little copying is better than a little dependency.",
	"Syscall must always be guarded with build tags.",
	"Cgo must always be guarded with build tags.",
	"Cgo is not Go.",
	"With the unsafe package there are no guarantees.",
	"Clear is better than clever.",
	"Reflection is never clear.",
	"Errors are values.",
	"Don't just check errors, handle them gracefully.",
	"Design the architecture, name the components, document the details.",
	"Documentation is for users.",
	"Don't panic."}

type Bucket struct {
	// Unix time for the bucket
	utime int64
	// message count
	count int
	// Slack RTM library that we are using doesn't give us the username of
	// the user sending the message, so we store only messages for now.
	msgs []string
}

type ByTimestamp []Bucket

func (a ByTimestamp) Len() int           { return len(a) }
func (a ByTimestamp) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByTimestamp) Less(i, j int) bool { return a[i].utime < a[j].utime }

type Counter struct {
	sync.RWMutex
	buckets []Bucket
	// Slack channel id for the channel this counter belongs to.
	ChannelId     string `json:"id"`
	meditationEnd time.Time
	messages      chan *slack.Msg

	// interval duration in minutes.
	Interval      string   `json:"interval"`
	MaxMsg        int      `json:"maxmsg"`
	SearchOver    []string `json:"search_over"`
	CreateTopicIn string   `json:"create_topic_in"`
}

func (c *Counter) MeditationEnd() time.Duration {
	c.RLock()
	defer c.RUnlock()
	return c.meditationEnd.Sub(time.Now())
}

func (c *Counter) SetMeditationEnd(d time.Duration) {
	c.Lock()
	defer c.Unlock()
	c.meditationEnd = time.Now().Add(d)
}

var meditateRegex, createRegex, queryRegex *regexp.Regexp

// Gives back the count of messages for the buckets which were created in the
// interval.
func (c *Counter) Count() int {
	sort.Sort(ByTimestamp(c.buckets))
	interval, err := time.ParseDuration(c.Interval)
	if err != nil {
		log.Fatalf("Got error while parsing duration. %s", err)
	}
	timeSince := time.Now().Add(-interval).Unix()
	idx := 0
	for i, b := range c.buckets {
		if b.utime > timeSince {
			idx = i
			break
		}
	}

	// We left shift the elements including the one at index idx to the
	// start of the bucket
	if idx > 0 {
		for i := idx; i < len(c.buckets); i++ {
			c.buckets[i-idx] = c.buckets[i]
		}
		c.buckets = c.buckets[0 : len(c.buckets)-idx]
	}

	count := 0
	for _, b := range c.buckets {
		count += b.count
	}
	return count
}

// Defining an interface so that these methods can be mocked easily while testing.
type RTM interface {
	SendMessage(msg *slack.OutgoingMessage)
	NewOutgoingMessage(text string, channel string) *slack.OutgoingMessage
}

func callYoda(c *Counter, rtm RTM, m string) {
	// Buckets set to nil after getting messages from it.
	c.buckets = nil
	msg := fmt.Sprintf("```%s\n%s\n%s```",
		string(yoda), proverbs[rand.Intn(len(proverbs))],
		m)
	rtm.SendMessage(rtm.NewOutgoingMessage(msg, c.ChannelId))
}

func discourseQuery(suffix string, args string) string {
	return fmt.Sprintf("%s/%s?api_key=%s&api_username=wisemonk&%s",
		conf.DiscPrefix, suffix, conf.DiscKey, args)
}

// Required fields for a discourse topic
type Topic struct {
	Title    string `json:"title"`
	Raw      string `json:"raw"`
	Category string `json:"category"`
}

// We need to extract these fields from the response that discourse sends
// when a topic is created successfully.
type TopicBody struct {
	Id   int    `json:"topic_id"`
	Slug string `json:"topic_slug"`
}

func topicUrl(tb TopicBody) string {
	return fmt.Sprintf("%s/t/%s/%d", conf.DiscPrefix, tb.Slug, tb.Id)
}

func sanitizeTitle(title string) string {
	t := strings.Trim(title, " ")
	// Discourse requires title to be atleast 20 chars.
	minLen := 20
	if len(t) < minLen {
		t = "Topic created by wisemonk with title: " + t
		return t
	}

	maxLen := 100
	// This is the max that discourse allows.
	if len(t) > maxLen {
		t = t[:maxLen]
	}
	// So that truncation happens at the last word break if possible.
	idx := strings.LastIndex(t, " ")
	if idx != -1 && idx >= minLen {
		t = t[:idx]
	}
	return t
}

func createTopic(c *Counter, title string) string {
	var buf bytes.Buffer

	buf.WriteString("```")
	count := 1
	for _, b := range c.buckets {
		for _, m := range b.msgs {
			fmt.Fprintf(&buf, "[%2d] %s\n", count, m)
			count++
		}
	}
	buf.WriteString("```")

	t := Topic{Title: title, Raw: buf.String(), Category: c.CreateTopicIn}
	bb := new(bytes.Buffer)
	json.NewEncoder(bb).Encode(t)
	q := discourseQuery("posts.json", "")
	res, err := http.Post(q, "application/json", bb)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusForbidden {
			log.Fatal("Discourse returned forbidden error.")
		}

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Topic: %v\nResponse status code: %d, body: %s",
			t, res.StatusCode, string(body))
		return ""
	}

	dec := json.NewDecoder(res.Body)
	var tb TopicBody
	err = dec.Decode(&tb)
	if err != nil {
		log.Fatal(err)
	}
	url := topicUrl(tb)
	return url
}

func sendMessage(c *Counter, rtm RTM) {
	msg := ""
	if conf.DiscKey == "" {
		callYoda(c, rtm, msg)
		return
	}
	// Picking the first message in the bucket as the discourse topic.
	title := sanitizeTitle(c.buckets[0].msgs[0])
	// The first message becomes the title.
	url := createTopic(c, title)
	// Incase we encountered an error from discourse, createTopic
	// would return an empty string as url.
	if url != "" {
		msg = fmt.Sprintf("Please move your discussion to %s", url)
	}
	callYoda(c, rtm, msg)
}

func substituteUsernames(text string, memmap map[string]string) string {
	userRegex, err := regexp.Compile(`<@U[A-Z0-9]{8}>`)
	if err != nil {
		log.Fatal(err)
	}

	res := userRegex.FindAllString(text, -1)
	if res == nil {
		return text
	}

	for _, u := range res {
		// extracting the userid
		uid := u[2 : len(u)-1]
		if uname, ok := memmap[uid]; ok {
			text = strings.Replace(text, u, "@"+uname, -1)
		}
	}
	return text
}

// Increment increases the count for a bucket or adds a new bucket with count 1
// to the Counter c
func (c *Counter) Increment(m *slack.Msg, memmap map[string]string) {
	if m.Channel != c.ChannelId {
		log.Fatalf("Channel mismatch, Expected: %s, Got: %s",
			c.ChannelId, m.Channel)
	}
	var tsf float64
	var err error
	if tsf, err = strconv.ParseFloat(m.Timestamp, 64); err != nil {
		log.Fatal(err)
	}
	ts := int64(tsf)
	m.Text = substituteUsernames(m.Text, memmap)
	msg := fmt.Sprintf("%-14s: %s", memmap[m.User], m.Text)

	// To check if a bucket for the timestamp already exists
	exists := false
	for i := len(c.buckets) - 1; i >= 0; i-- {
		b := &c.buckets[i]
		if b.utime == ts {
			b.count++
			b.msgs = append(b.msgs, msg)
			exists = true
			break
		}
	}

	if exists != true {
		c.buckets = append(c.buckets, Bucket{utime: ts, count: 1,
			msgs: []string{msg}})
	}
}

// This method listens for incoming events. It puts message events onto
// a channel
func listen(rtm *slack.RTM) {
	// This has been mostly picked up from
	// https://github.com/nlopes/slack/blob/master/examples/websocket/websocket.go
	for {
		msg := <-rtm.IncomingEvents
		switch ev := msg.Data.(type) {
		case *slack.ConnectedEvent:
		case *slack.MessageEvent:
			if sm, ok := msg.Data.(*slack.MessageEvent); ok {
				// Putting the message on the Counter it belongs
				// to
				m := sm.Msg

				if c, ok := conf.Channels[m.Channel]; ok {
					c.messages <- &m
				}
			}
		case *slack.RTMError:
			log.Fatal(ev.Error())
		case *slack.InvalidAuthEvent:
			log.Fatal(errors.New("Invalid credentails"))
		}
	}
}

// This function checks if wisemonk was asked to create a topic. If he ways,
// it creates a new topic and returns its url.
func createNewTopic(c *Counter, m string, rtm RTM) {
	if conf.DiscKey == "" {
		return
	}

	res := createRegex.FindStringSubmatch(m)
	if res == nil {
		return
	}

	title := sanitizeTitle(res[1])
	url := createTopic(c, title)
	c.buckets = nil

	msg := "New topic created with url: " + url
	rtm.SendMessage(rtm.NewOutgoingMessage(msg,
		c.ChannelId))
}

// This function checks if wisemonk was asked to meditate by matching the
// message against a regex. If the message was a valid command then wisemonk
// stops sending messages for the specified duration.
func askToMeditate(c *Counter, m string) string {
	res := meditateRegex.FindStringSubmatch(m)
	if res == nil {
		return ""
	}

	// Captured time is available at the first index. The duration can be
	// anything that time.ParseDuration accepts.
	d, err := time.ParseDuration(res[1])
	if err != nil {
		return "Sorry, I don't understand you."
	}

	if d < 0 {
		return "Sorry, going back in time is not what I can do."
	}

	if d >= time.Hour {
		return "It's hard to meditate for more than an hour at one go you know."
	}

	if d := c.MeditationEnd(); d > 0 {
		return fmt.Sprintf("I am meditating. My meditation will finish in %.0f mins",
			d.Minutes())
	}

	c.SetMeditationEnd(d)
	go func() {
		time.Sleep(d)
		// We clear the buckets when wisemonk wakes up from his meditation.
		c.buckets = nil
		// TODO(pawan) - Send message when wisemonk has ended his
		// meditation.

	}()
	return fmt.Sprintf("Okay, I am going to meditate for %s", d)
}

type SearchTopic struct {
	Id       int    `json:"id"`
	Slug     string `json:"slug"`
	Category int    `json:"category_id"`
}

type SearchResponse struct {
	Topics []SearchTopic `json:"topics"`
}

func filterTopics(c *Counter, topics []SearchTopic) []SearchTopic {
	var filteredTopics []SearchTopic
	for idx, t := range topics {
		keep := false
		for _, cat := range c.SearchOver {
			keep = false
			if discourseCategory[t.Category] == cat {
				keep = true
				break
			}
		}
		if keep {
			filteredTopics = append(filteredTopics, topics[idx])
		}
	}
	return filteredTopics
}

func searchDiscourse(c *Counter, m string, rtm RTM) {
	if conf.DiscKey == "" {
		return
	}
	res := queryRegex.FindStringSubmatch(m)
	if res == nil {
		return
	}

	query := res[1]
	maxResults, err := strconv.Atoi(res[2])
	if err != nil {
		rtm.SendMessage(rtm.NewOutgoingMessage("Sorry, I didn't understand you.",
			c.ChannelId))
		return
	}

	q := discourseQuery("search.json", fmt.Sprintf("q=%s&order=%s",
		url.QueryEscape(query), "views"))

	var sr SearchResponse
	runQueryAndParseResponse(q, &sr)
	sr.Topics = filterTopics(c, sr.Topics)
	// Picking just the top 3 topics
	if len(sr.Topics) > maxResults {
		sr.Topics = sr.Topics[:maxResults]
	}
	var buf bytes.Buffer
	for _, t := range sr.Topics {
		buf.WriteString(fmt.Sprintf("%s/t/%s/%d\n", conf.DiscPrefix,
			t.Slug, t.Id))
	}
	if buf.Len() > 0 {
		rtm.SendMessage(rtm.NewOutgoingMessage(buf.String(), c.ChannelId))
	} else {
		rtm.SendMessage(rtm.NewOutgoingMessage("Sorry, I didn't find anything.",
			c.ChannelId))
	}
}

func (c *Counter) checkOrIncr(rtm *slack.RTM, wg sync.WaitGroup,
	memmap map[string]string) {
	defer wg.Done()
	ticker := time.NewTicker(time.Second * 10)

	for {
		select {
		case msg := <-c.messages:
			searchDiscourse(c, msg.Text, rtm)
			createNewTopic(c, msg.Text, rtm)
			m := askToMeditate(c, msg.Text)
			if m != "" {
				rtm.SendMessage(rtm.NewOutgoingMessage(m,
					c.ChannelId))
			}
			// If we receive a message on the channel, we increment
			// the counter.
			c.Increment(msg, memmap)
		case <-ticker.C:
			// We perform this check only if the monk is not meditating.
			if d := c.MeditationEnd(); d < 0 {
				count := c.Count()
				if count >= c.MaxMsg {
					go sendMessage(c, rtm)
				}
			}
		}
	}
}

type Members struct {
	Users []Member `json:"members"`
}

type Member struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

func runQueryAndParseResponse(q string, data interface{}) {
	resp, err := http.Get(q)
	if err != nil {
		log.Fatalf("Url: %s. Error: %v", q, err)
	}

	if resp.StatusCode != 200 {
		log.Fatalf("Url: %s. Status: %v", q, resp.Status)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Url: %s. Error: %v", q, err)
	}

	if err := json.Unmarshal(body, data); err != nil {
		log.Fatalf("Url: %s. Error: %v", q, err)
	}
}

func slackQuery(suffix string) string {
	return fmt.Sprintf("%s/%s?token=%s", slackPrefix, suffix, conf.Token)
}

func cacheUsernames(url string) map[string]string {
	memmap := make(map[string]string)
	var m Members

	runQueryAndParseResponse(url, &m)
	for _, u := range m.Users {
		memmap[u.Id] = u.Name
	}
	return memmap
}

type CategoryRes struct {
	CategoryList Categories `json:"category_list"`
}

type Categories struct {
	Cats []Category `json:"categories"`
}

type Category struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

var discourseCategory map[int]string

func cacheCategories(url string) {
	if conf.DiscKey == "" {
		return
	}

	var cr CategoryRes
	discourseCategory = make(map[int]string)

	runQueryAndParseResponse(url, &cr)
	for _, c := range cr.CategoryList.Cats {
		discourseCategory[c.Id] = c.Name
	}
	checkDiscourseCategory(conf.Channels, url)
}

// Checks if the discourse Category supplied as flag exists. If not
// it logs error and exits.
func checkDiscourseCategory(channels map[string]*Counter, url string) {
	for _, channel := range channels {
		exists := false
		for _, cname := range discourseCategory {
			if cname == channel.CreateTopicIn {
				exists = true
				break
			}
		}
		if !exists {
			log.Fatalf("Category %s doesn't exist in discourse.",
				channel.CreateTopicIn)
		}
	}
}

func init() {
	var err error
	yoda, err = ioutil.ReadFile("yoda.txt")
	if err != nil {
		log.Fatal(err)
	}
	// We capture the duration using a capturing group.
	meditateRegex, err = regexp.Compile(`wisemonk meditate for (.+)`)
	if err != nil {
		log.Fatal(err)
	}
	createRegex, err = regexp.Compile(`wisemonk create topic (.+)`)
	if err != nil {
		log.Fatal(err)
	}
	queryRegex, err = regexp.Compile(`wisemonk query (.+) (\d)`)
	if err != nil {
		log.Fatal(err)
	}
	readConfig("config.json")
}

type Config struct {
	Token      string              `json:"token"`
	DiscPrefix string              `json:"discourseprefix"`
	DiscKey    string              `json:"discoursekey"`
	Channels   map[string]*Counter `json:"channels"`
}

var conf Config

func readConfig(filename string) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("Error reading file, %s. %s", filename, err)
	}

	conf.Channels = make(map[string]*Counter)
	err = json.Unmarshal(b, &conf)
	if err != nil {
		log.Fatalf("Error while unmarshaling data from config while. %s",
			err)
	}
}

func main() {
	flag.Parse()
	cacheCategories(discourseQuery("categories.json", ""))
	readConfig("config.json")
	api := slack.New(conf.Token)
	api.SetDebug(false)
	rtm := api.NewRTM()
	go rtm.ManageConnection()

	var wg sync.WaitGroup
	// Map of slack userids to usernames.
	memmap := cacheUsernames(slackQuery("users.list"))

	for cid, c := range conf.Channels {
		wg.Add(1)
		c.messages = make(chan *slack.Msg, 500)
		c.ChannelId = cid
		go c.checkOrIncr(rtm, wg, memmap)
	}
	go listen(rtm)
	wg.Wait()
}
