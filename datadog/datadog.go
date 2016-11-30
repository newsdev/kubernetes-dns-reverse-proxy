package datadog

import (
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/DataDog/datadog-go/statsd"
)

var (
	dataDogHostPort = os.Getenv("DD_AGENT_SERVICE_HOST_PORT")
	client, err     = statsd.New(dataDogHostPort) //in host:port format
)

func init() {
	if err != nil {
		log.Error("Couldn't connect to statsd", dataDogHostPort, err)
	} else {
		log.Info("Connected to statsd", dataDogHostPort)
		// prefix every metric with the app name
		client.Namespace = "k8s_dns."
		// send the pod hostname as a tag with every metric
		client.Tags = append(client.Tags, os.Getenv("ENVIRONMENT"))
	}
}

func Count(name string, value int64, tags []string, rate float64) {
	//See init(). If connecting to DD-Agent failed, err is not nil
	if (err != nil) {
		return
	}
	countError := client.Count(name, value, tags, rate)
	if countError != nil {
		log.Error("Error sending metrics to DataDog:", countError)
	}
}
