/*
 * Copyright 2016 DGraph Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * 		http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nlopes/slack"
)

func TestSanitizeTitle(t *testing.T) {
	title := "Short title"
	expected := "Topic created by wisemonk with title: Short title"
	if st := sanitizeTitle(title); st != expected {
		t.Errorf("Expected: %s, Got: %s", expected, st)
	}

	title = "Long title with word breaks"
	expected = "Long title with word"
	if st := sanitizeTitle(title); st != expected {
		t.Errorf("Expected: %s, Got: %s", expected, st)
	}

	title = "This title is 20char"
	expected = title
	if st := sanitizeTitle(title); st != expected {
		t.Errorf("Expected: %s, Got: %s", expected, st)
	}

	title = `This title has more than 100chars. It should be trimmed
	down. We should avoid having long titles obviously`
	expected = `This title has more than 100chars. It should be trimmed
	down. We should avoid having long titles`
	if st := sanitizeTitle(title); st != expected {
		t.Errorf("Expected: %s, Got: %s", expected, st)
	}

	title = "          Short title"
	expected = "Topic created by wisemonk with title: Short title"
	if st := sanitizeTitle(title); st != expected {
		t.Errorf("Expected: %s, Got: %s", expected, st)
	}
}

func TestAskToMeditate(t *testing.T) {
	c := &Counter{}

	message := "wisemonk meditat for 1hr"
	m := askToMeditate(c, message)
	em := ""
	if m != em {
		t.Errorf("Expected: %s, Got: %s", em, m)
	}

	message = "wisemonk meditate for 1hr"
	m = askToMeditate(c, message)
	em = "Sorry, I don't understand you."
	if m != em {
		t.Errorf("Expected: %s, Got: %s", em, m)
	}

	message = "wisemonk meditate for 200h"
	m = askToMeditate(c, message)
	em = "It's hard to meditate for more than an hour at one go you know."
	if m != em {
		t.Errorf("Expected: %s, Got: %s", em, m)
	}

	message = "wisemonk meditate for -5m"
	m = askToMeditate(c, message)
	em = "Sorry, going back in time is not what I can do."
	if m != em {
		t.Errorf("Expected: %s, Got: %s", em, m)
	}

	message = "wisemonk meditate for 5m"
	m = askToMeditate(c, message)
	em = "Okay, I am going to meditate for 5m0s"
	if m != em {
		t.Errorf("Expected: %s, Got: %s", em, m)
	}

	message = "wisemonk meditate for 5m"
	m = askToMeditate(c, message)
	em = "I am meditating. My meditation will finish in 5 mins"
	if m != em {
		t.Errorf("Expected: %s, Got: %s", em, m)
	}
}

func TestIncrement(t *testing.T) {
	c := &Counter{ChannelId: "general"}
	msgs := []slack.Msg{
		{Channel: "general", Timestamp: "1465010249.000606",
			Text: " First message"},
		{Channel: "general", Timestamp: "1465010259.000606",
			Text: " Second message"},
		{Channel: "general", Timestamp: "1465010249.000806",
			Text: " Third message at same timestamp as first"},
	}

	for _, m := range msgs {
		c.Increment(&m, map[string]string{})
	}
	if len(c.buckets) != 2 {
		t.Errorf("Expected: %d,Got: %d buckets", 1, len(c.buckets))
	}
	if c.buckets[0].count != 2 {
		t.Errorf("Expected bucket to have %d messages, Got: %d", 2,
			c.buckets[0].count)
	}
	if c.buckets[1].count != 1 {
		t.Errorf("Expected bucket to have %d messages, Got: %d", 1,
			c.buckets[1].count)
	}
}

func addBuckets(c *Counter, text string, t int64) {
	for i := 0; i < 10; i++ {
		c.Increment(&slack.Msg{Channel: "general",
			Timestamp: strconv.FormatInt(t-int64(i), 10),
			Text:      text}, map[string]string{})
	}
}

func TestCount(t *testing.T) {
	c := &Counter{ChannelId: "general", Interval: "10m"}
	timeNow := time.Now().Unix()
	addBuckets(c, "New buckets", timeNow)

	timeBfrInterval := time.Now().Add(-10 * time.Minute).Unix()
	addBuckets(c, "Old buckets", timeBfrInterval)
	if count := c.Count(); count != 10 {
		t.Errorf("Expected count to be %d, Got: %d", 10, count)
	}
	if len(c.buckets) != 10 {
		t.Errorf("Expected %d buckets, Got: %d", 10, len(c.buckets))
	}
}

func createServer(t *testing.T, status int, i interface{}) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		r *http.Request) {
		w.WriteHeader(status)
		b, err := json.Marshal(i)
		if err != nil {
			t.Error(err)
		}
		w.Write(b)
	}))
	return ts
}

func TestCreateTopic(t *testing.T) {
	c := &Counter{ChannelId: "general"}
	timeNow := time.Now().Unix()
	addBuckets(c, "New buckets", timeNow)

	ts := createServer(t, http.StatusNotFound, TopicBody{})
	conf.DiscPrefix = ts.URL
	defer ts.Close()

	if url := createTopic(c, "Test title"); url != "" {
		t.Errorf("Expected url to be blank, Got: ", url)
	}

	ts = createServer(t, http.StatusOK,
		TopicBody{Id: 1, Slug: "test-title-created"})
	conf.DiscPrefix = ts.URL
	if url := createTopic(c, "Test title"); !strings.Contains(url,
		"test-title-created") {
		t.Errorf("Expected url to contain test-title-created, Got: %s",
			url)
	}
}

type r struct {
}

var invoked = false

func (rtm *r) SendMessage(msg *slack.OutgoingMessage) {
	invoked = true
}

func (rtm *r) NewOutgoingMessage(text string, channel string) *slack.OutgoingMessage {
	return new(slack.OutgoingMessage)
}

func TestSearchDiscourse(t *testing.T) {
	c := &Counter{ChannelId: "general"}
	rtm := &r{}
	invoked = false
	conf.DiscKey = "testkey"
	ts := createServer(t, http.StatusOK,
		SearchResponse{Topics: []SearchTopic{
			{Id: 1, Slug: "test-1"},
			{Id: 2, Slug: "test-2"},
		}})
	conf.DiscPrefix = ts.URL
	if searchDiscourse(c, "wisemonk search something", rtm); invoked {
		t.Errorf("rtm.SendMessage() should not have been called")
	}

	if searchDiscourse(c, "wisemonk query test", rtm); !invoked {
		t.Errorf("rtm.SendMessage() should have been called")
	}
}

func TestCallYoda(t *testing.T) {
	c := &Counter{ChannelId: "general"}
	timeNow := time.Now().Unix()
	addBuckets(c, "New buckets", timeNow)
	rtm := &r{}

	if callYoda(c, rtm, "Message to append"); !invoked {
		t.Errorf("Expected invoked to be %t, Got: %t", true, false)
	}
}

func TestSendMessage(t *testing.T) {
	c := &Counter{ChannelId: "general"}
	timeNow := time.Now().Unix()
	addBuckets(c, "New buckets", timeNow)
	rtm := &r{}

	invoked = false
	if sendMessage(c, rtm); !invoked {
		t.Errorf("Expected invoked to be %t, Got: %t", true, false)
	}

	conf.DiscKey = "testkey"
	addBuckets(c, "New buckets", timeNow)
	invoked = false
	ts := createServer(t, http.StatusOK, TopicBody{Id: 1,
		Slug: "test-title-created"})
	conf.DiscPrefix = ts.URL
	defer ts.Close()

	if sendMessage(c, rtm); !invoked {
		t.Errorf("Expected invoked to be %t, Got: %t", true, false)
	}
}

func TestCreateNewTopic(t *testing.T) {
	c := &Counter{ChannelId: "general"}
	timeNow := time.Now().Unix()
	addBuckets(c, "New buckets", timeNow)
	m := "wisemonk create topic testing wisemonk"
	rtm := &r{}
	ts := createServer(t, http.StatusOK, TopicBody{Id: 1,
		Slug: "test-title-created"})
	conf.DiscPrefix = ts.URL
	defer ts.Close()

	conf.DiscKey = "testkey"
	invoked = false
	if createNewTopic(c, m, rtm); !invoked {
		t.Errorf("Expected invoked to be %t, Got: %t", true, false)
	}
}

func TestSubstituteUsernames(t *testing.T) {
	memmap := make(map[string]string)
	memmap["U13LHF42F"] = "mrjn"
	memmap["U13LHF42G"] = "pawan"
	text := "<@U13LHF42F> <@U13LHF42F> <@U13LHF42G>"

	text = substituteUsernames(text, memmap)
	expected := "@mrjn @mrjn @pawan"
	if text != expected {
		t.Errorf("Expected %s, Got: %s", expected, text)
	}
}

func TestRunQueryAndParseResponse(t *testing.T) {
	mems := Members{}
	mems.Users = append(mems.Users, Member{Id: "U13GH76YT", Name: "mrjn"},
		Member{Id: "U13GH13YT", Name: "pawan"})
	ts := createServer(t, http.StatusOK, mems)
	defer ts.Close()

	var m Members
	runQueryAndParseResponse(ts.URL, &m)
}

func TestCacheUsernames(t *testing.T) {
	mems := Members{}
	mems.Users = append(mems.Users, Member{Id: "U13GH76YT", Name: "mrjn"},
		Member{Id: "U13GH13YT", Name: "pawan"})
	ts := createServer(t, http.StatusOK, mems)
	defer ts.Close()

	memmap := cacheUsernames(ts.URL)
	if _, ok := memmap["U13GH76YT"]; !ok {
		t.Errorf("Expected ok to be true. Got false")
	}
	uname := memmap["U13GH76YT"]
	if uname != "mrjn" {
		t.Errorf("Expected username to be mrjn, Got: ", uname)
	}
	if _, ok := memmap["U13GH13YT"]; !ok {
		t.Errorf("Expected ok to be true. Got false")
	}
}

func TestCheckDiscourseCategory(t *testing.T) {
	readConfig("config_test.json")
	discourseCategory = make(map[int]string)
	discourseCategory[1] = "Slack"
	discourseCategory[2] = "Users"
	cr := CategoryRes{CategoryList: Categories{}}
	cr.CategoryList.Cats = append(cr.CategoryList.Cats,
		Category{Name: "Slack"},
		Category{Name: "Users"})
	ts := createServer(t, http.StatusOK, cr)
	defer ts.Close()

	checkDiscourseCategory(conf.Channels, ts.URL)
}

func TestReadConfig(t *testing.T) {
	readConfig("config_test.json")
	if conf.Token == "" {
		t.Errorf("Expected token to not be nil.")
	}
	if conf.DiscPrefix == "" {
		t.Errorf("Expected discuss prefix to not be nil.")
	}
	if len(conf.Channels) != 2 {
		t.Errorf("Expected len of Channels to be %d. Got: %d", 2,
			len(conf.Channels))
	}
}
