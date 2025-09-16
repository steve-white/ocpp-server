package telemetry

import (
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	logrus_appinsights "github.com/steve-white/logrus-appinsights"
)

// AppInsightsHook is a logrus hook for Application Insights
type AppInsightsHook struct {
	Client appinsights.TelemetryClient

	async        bool
	levels       []logrus.Level
	ignoreFields map[string]struct{}
	filters      map[string]func(interface{}) interface{}
}

var client appinsights.TelemetryClient

// New returns an initialised logrus hook for Application Insights
func NewTelemetryClient(instrumentationKey string, roleName string) (*logrus_appinsights.AppInsightsHook, error) {
	if len(instrumentationKey) == 0 {
		return nil, nil
	}

	hook, err := logrus_appinsights.New("my_client", logrus_appinsights.Config{
		InstrumentationKey: instrumentationKey,
		MaxBatchSize:       10,              // optional
		MaxBatchInterval:   time.Second * 5, // optional
	})
	if err != nil || hook == nil {
		panic(err)
	}

	// set custom levels
	hook.SetLevels([]log.Level{
		log.PanicLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
		log.DebugLevel,
	})
	// ignore fields
	//hook.AddIgnore("private")
	client = hook.Client
	return hook, nil
}

func TrackConnectionRequest(url string, timeMs int) {
	if client == nil {
		return
	}
	var duration time.Duration = time.Duration(timeMs) * time.Millisecond
	client.TrackRequest("GET", url, duration, "302")
}

func TrackAuthenticationEvent(networkId string, clientAddress string, responseCode string) {
	if client == nil {
		return
	}

	event := appinsights.NewEventTelemetry("AuthenticationEvent")
	event.Properties["networkId"] = networkId
	event.Properties["clientAddress"] = clientAddress
	event.Properties["responseCode"] = responseCode
	client.Track(event)
}

func TrackOcppRequest(networkId string, clientAddress string, ocppMsgId string, msgType string, responseCode string, duration time.Duration) {
	if client == nil {
		return
	}

	//	durationMs := requestEndTime.Sub(requestStartTime)
	request := appinsights.NewRequestTelemetry("GET", msgType, duration, responseCode)

	// Note that the timestamp will be set to time.Now() minus the
	// specified duration.  This can be overridden by either manually
	// setting the Timestamp and Duration fields, or with MarkTime:
	//request.MarkTime(requestStartTime, requestEndTime)

	request.Source = clientAddress
	//request.Success = false

	request.Properties["ocppMsgId"] = ocppMsgId
	//request.Properties["user-agent"] = request.headers["User-agent"]
	//request.Measurements["POST size"] = float64(len(data))

	// Context tags become more useful here as well
	//request.Tags.Session().SetId("<session id>")
	//request.Tags.User().SetAccountId("<user id>")

	// Finally track it
	client.Track(request)
}

func TrackTraceVerbose(message string) {
	if client == nil {
		return
	}
	trace := appinsights.NewTraceTelemetry(message, appinsights.Verbose)
	client.Track(trace)
}

func TrackTraceInformation(message string) {
	if client == nil {
		return
	}
	trace := appinsights.NewTraceTelemetry(message, appinsights.Information)
	client.Track(trace)
}

func TrackTraceWarning(message string) {
	if client == nil {
		return
	}
	trace := appinsights.NewTraceTelemetry(message, appinsights.Warning)
	client.Track(trace)
}

func TrackTraceError(message string) {
	if client == nil {
		return
	}
	trace := appinsights.NewTraceTelemetry(message, appinsights.Error)
	client.Track(trace)
}

func TrackTraceCritical(message string) {
	if client == nil {
		return
	}
	trace := appinsights.NewTraceTelemetry(message, appinsights.Critical)
	client.Track(trace)
}
