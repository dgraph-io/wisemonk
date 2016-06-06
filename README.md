# Wisemonk

Wisemonk is a slackbot written in Go that listens to messages on slack and responds to them via the [Slack RTM API](https://api.slack.com/rtm).

Wisemonk isn't just another bot, its a super intelligent one. You can tell it what channels to monitor and set a message limit in a specified duration. It will alert you when the number of messages exchanged exceed the max message limit set by you in the given duration.

## Usage

After cloning the repo, you can build the binary and run wisemonk like this.

`./wisemonk -token="bot-user-token" -channels="G1D59039B,G1D6B4T6Z" -discoursekey="discourse-api-key" discourseprefix="https://discuss.dgraph.io" -interval=20*time.Minute -maxmsg=50
`
So now if in any 20 minute interval more than 50 messages are exchanged, wisemonk would alert you.

Token for slack can be obtained after creating a bot user at https://api.slack.com/bot-users. Also note that you would have to add wisemonk as a user to all the channels that you want it to be active on.

If you use [discourse](https://www.discourse.org/), then wisemonk has some other advanced functionalities that you could make use of. Wisemonk stores the messages exchanged and automatically creates a discourse topic for you, a link of which it shares while sending the alert.


You could customize the alert message displayed. For now we display Yoda, followed by a [Go Proverb](https://go-proverbs.github.io/) and then the link for the discourse topic if a discourse key and discourse prefix are given as flags.

```
Usage of ./wisemonk:
  -channels string
        Comma separated ids for slack channels on which to activate the bot.
  -discoursekey string
        API key used to authenticate requests to discourse.
  -discourseprefix string
        Prefix for api communication with discourse.
  -interval duration
        Interval size to monitor in minutes. (default 10m0s)
  -maxmsg int
        Max messages allowed in the interval. (default 20)
  -token string
        Slack auth token for the bot user.
```

## Interaction

Sometimes you are having an important discussion on slack and don't want wisemonk to interrupt you. In these scenarios you could ask the wisemonk to meditate for some time like this in your slack channel.

`wisemonk meditate for 20m`

If successful, wisemonk replies with `Okay, I am going to meditate for 20m`. The duration can be anything understood by [ParseDuration](https://golang.org/pkg/time/#ParseDuration).

If you are using discourse and you observe that you are having an important discussion, you could create a discourse topic from slack using wisemonk. This topic would have your last n messages and would provide relevant context for further discussion on discourse. The command for creating a topic is

`create topic [title of discourse topic]`

Wisemonk will reply back with the url of the new topic that was created.
