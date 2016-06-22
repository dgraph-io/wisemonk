# Wisemonk

Wisemonk is a slackbot to keep people from wasting too much time on Slack. It isn't just another bot, its a super intelligent one. You can tell it what channels to monitor and then it can

* alert you when you are talking too much.
* automatically create a topic on discourse with the contents of your chat and share the url with you.
* create a topic on demand on discourse.

A lot of teams have found that the synchronous nature of communication on slack destroys the productivity of their team members.For more on that read this article [here](https://medium.com/better-people/slack-i-m-breaking-up-with-you-54600ace03ea#.ox3a8tukc). Thus we at Dgraph try to have most of our structured and meaningful conversations on [Discourse](https://www.discourse.org/). Wisemonk constantly monitors our communication and helps us move our discussion to discourse when we are talking a lot.

## Install

`go get github.com/dgraph-io/wisemonk`

`go install github.com/dgraph-io/wisemonk`

We use govendor to manage our deps and all our deps our checked in to the repo so you don't need to install them separately.

## Usage

Add config to `config.json`.

```
{
  // slackbot token.
  "token": "",
  "discourseprefix": "https://discuss.dgraph.io",
  // discourse api key.
  "discoursekey": "",
  "channels": {
      // slack channel id
      "G1D59039B": {
        // interval should be a value that can be parsed by https://golang.org/pkg/time/#ParseDuration.
        "interval": "10m",
        "maxmsg":20,
        // slug of discourse categories that wisemonk would search in.
        "search_over":["minions","dev","user","decisions","reading","blog","faqs","docs","use-cases","recruit"],
        // slug of discourse category that a new topic would be created in.
        "create_topic_in": "slack"
      },
    }
}
```
After adding config, since the wisemonk binary is now installed and if you have `$GOPATH/bin` in your path you can call wisemonk like this `wisemonk`

Now if in any 10 minute interval more than 20 messages are exchanged, wisemonk would alert you.

Token for slack can be obtained after creating a bot user at https://api.slack.com/bot-users. Also note that you would have to add wisemonk as a user to all the channels that you want it to be active on.

If you use [discourse](https://www.discourse.org/), then wisemonk has some other advanced functionalities that you could make use of. Wisemonk stores the messages exchanged and automatically creates a discourse topic for you, a link of which it shares while sending the alert.


You could customize the alert message displayed. For now we display Yoda, followed by a [Go Proverb](https://go-proverbs.github.io/) and then the link for the discourse topic if a discourse key and discourse prefix are given as config.


## Interaction

- You can search over your topics in discourse like

  `wisemonk query [query_string] [max_count]`

  So `wisemonk query release v0.3 5` would return url of top 5 topics which have `release v0.3` as part of them.

- Sometimes you are having an important discussion on slack and don't want wisemonk to interrupt you. In these scenarios you could ask the wisemonk to meditate for some time like this in your slack channel.

  `wisemonk meditate for 20m`

  If successful, wisemonk replies with `Okay, I am going to meditate for 20m`. The duration can be anything understood by [ParseDuration](https://golang.org/pkg/time/#ParseDuration).

- If you are using discourse and you observe that you are having an important discussion, you could create a discourse topic from slack using wisemonk. This topic would have your last n messages and would provide relevant context for further discussion on discourse. The command for creating a topic is

  `wisemonk create topic [title of discourse topic]`

  Wisemonk will reply back with the url of the new topic that was created.

## Technologies involved

Wisemonk is written in Go and makes use of

* [Slack RTM API](https://api.slack.com/rtm)
* [Discourse API](https://meta.discourse.org/t/discourse-api-documentation/22706)

## About the project
Wisemonk was born out of our experience at [Dgraph](https://github.com/dgraph-io/dgraph). Read more about why we built it in our [blogpost](https://medium.dgraph.io/wisemonk-a-slackbot-to-move-discussions-from-slack-to-discourse-22a53ddce78f#.rcn1wlv3p).

We got featured on [Hacker News](https://news.ycombinator.com/item?id=11908704) too.

