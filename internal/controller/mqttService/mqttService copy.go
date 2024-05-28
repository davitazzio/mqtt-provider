package mqttservice

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// A mqttservice does nothing.

func StartBroker(remoteHost string, remoteUser string, logger logging.Logger) (string, error) {

	client, session, err := connectToHost(remoteUser, remoteHost)
	if err != nil {
		logger.Debug("errore CONNESSIONE")
		logger.Debug(err.Error())
		return "", err
	}

	// copia del file
	command_str := fmt.Sprintf("scp -r dtazzioli@dtazzioli-processprovider.cloudmmwunibo.it:/home/dtazzioli/hivemq-4.28.0 /home/%s", remoteUser)
	logger.Debug(command_str)
	result, err := session.CombinedOutput(command_str)
	logger.Debug(string(result))
	if err != nil {
		logger.Debug("errore COPIA")
		logger.Debug(err.Error())
		return "", err
	}
	client.Close()

	client, session, err = connectToHost(remoteUser, remoteHost)
	if err != nil {
		logger.Debug("errore CONNESSIONE")
		logger.Debug(err.Error())
		return "", err
	}

	output, err := session.CombinedOutput(fmt.Sprintf("screen -d -m /home/%s/hivemq-4.28.0/bin/run.sh", remoteUser))
	if err != nil {
		logger.Debug("errore nell'avvio dell'applicazione")
		logger.Debug(err.Error())
		return "", err
	}

	client.Close()

	return string(output), nil

}

func BrokerExist(remoteHost string, remoteUser string, logger logging.Logger) bool {
	client, session, err := connectToHost(remoteUser, remoteHost)
	if err != nil {
		logger.Debug("errore CONNESSIONE")
		logger.Debug(err.Error())
		return false
	}

	output, err := session.CombinedOutput("pgrep -f hive")
	if err != nil {
		logger.Debug("errore grep")
		logger.Debug(err.Error())
		return false
	}
	logger.Debug(string(output))
	client.Close()
	return string(output) != ""
}
func Observebroker(remoteHost string, remoteUser string, logger logging.Logger) (int, error) {

	resp, err := http.Get(fmt.Sprintf("http://%s:9399/metrics", remoteHost))
	if err != nil {
		logger.Debug("errore CONNESSIONE")
		logger.Debug(err.Error())
		return -1, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	var queue_str = "-1"
	for _, line := range strings.Split(string(body), "\n") {
		// fmt.Printf(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		words := strings.Split(line, " ")
		// fmt.Print(words[0])
		if words[0] == "com_hivemq_messages_client_queued_count" {
			queue_str = words[1]
		}

	}
	queue, _ := strconv.ParseFloat(queue_str, 64)
	// logger.Debug("LETTA LA DIMENSIONE DELLA CODA: ")
	// logger.Debug(fmt.Sprintf("%d", int(queue)))
	return int(queue), nil

}

func TerminateBroker(remoteHost string, remoteUser string, logger logging.Logger) error {

	client, session, err := connectToHost(remoteUser, remoteHost)
	if err != nil {
		logger.Debug("errore CONNESSIONE")
		logger.Debug(err.Error())
		return err
	}

	output, err := session.CombinedOutput("pgrep -f hive")
	if err != nil {
		logger.Debug("errore grep")
		logger.Debug(err.Error())
		return err
	}

	process_pid, _ := strconv.Atoi(string(output))

	_, err = session.CombinedOutput(fmt.Sprintf("kill %d", process_pid))
	if err.Error() == "Process exited with status 143 from signal TERM" {
		client.Close()

		return nil
	} else {
		logger.Debug("errore kill")
		logger.Debug(err.Error())
		client.Close()
		return err
	}
}
