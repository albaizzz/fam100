package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/magiconair/properties"
	"github.com/spf13/cobra"
	"github.com/uber-go/zap"
	"github.com/yulrizka/bot"
	"github.com/yulrizka/bot-play"
)

var (
	ctx context.Context
	p   *properties.Properties
	log zap.Logger

	roundSeconds time.Duration
	startedAt    time.Time

	// bot configuration
	botInBuffer  int64
	botOutBuffer int64

	// graphite
	graphiteURL    string
	graphiteWebURL string
)

// compiled time information
var (
	VERSION   = ""
	BUILDTIME = ""
)

func main() {
	ctx = context.TODO()
	p = properties.NewProperties()
	log = zap.New(zap.NewJSONEncoder(), zap.AddCaller(), zap.AddStacks(zap.FatalLevel))

	roundSeconds = p.GetParsedDuration("roundSeconds", 90*time.Second)
	botInBuffer = p.GetInt64("bot.in.bufferSize", 10000)
	botOutBuffer = p.GetInt64("bot.out.bufferSize", 10000)

	play.GameQuorum = p.GetInt("game.quorum", 3)

	graphiteURL = p.GetString("graphite.url", "")
	graphiteWebURL = p.GetString("graphite.web.url", "")

	http.DefaultClient.Timeout = p.GetParsedDuration("http.timeout", 10*time.Second)

	rootCmd := &cobra.Command{
		Use:   "fam100",
		Short: "Telegram fam100 bot",
		Run:   mainFn,
	}
	if err := rootCmd.Execute(); err != nil {
		log.Fatal("main failed", zap.Error(err))
	}
}

func mainFn(cmd *cobra.Command, args []string) {
	log.Info("Fam100 STARTED",
		zap.String("version", VERSION),
		zap.String("buildtime", BUILDTIME),
	)
	postEvent("startup", "startup", fmt.Sprintf("startup version:%s buildtime:%s", VERSION, BUILDTIME))

	key := os.Getenv("TELEGRAM_KEY")
	if key == "" {
		log.Fatal("TELEGRAM_KEY can not be empty")
	}

	startedAt = time.Now()
	telegram, err := bot.NewTelegram(key)
	if err != nil {
		log.Fatal("failed to start telegram", zap.Error(err))
	}
	if err := telegram.AddPlugin(&fam100{}); err != nil {
		log.Fatal("Failed AddPlugin", zap.Error(err))
	}

	telegram.Start()
}

type fam100 struct {
	in  chan interface{}
	out chan bot.Message

	manager *play.Manager
	client  bot.Client
}

func (*fam100) Name() string {
	return "fam100"
}

func (b *fam100) Init(out chan bot.Message) (in chan interface{}, err error) {
	b.in = make(chan interface{}, botInBuffer)
	b.out = out

	return b.in, nil
}
