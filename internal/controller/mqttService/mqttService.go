package mqttservice

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// A mqttservice does nothing.
type Mqttservice struct {
	RemoteHost string
}

type mqttserviceList struct {
	items []*Mqttservice
}

func (pl *mqttserviceList) GetItems() []*Mqttservice {
	return pl.items
}

func (pl *mqttserviceList) AddService(service *Mqttservice) {
	pl.items = append(pl.items, service)

}

var instances *mqttserviceList

func GetInstance(remoteHost string) *Mqttservice {

	// Pattern singleton for each broker created
	if instances == nil {
		instances = &mqttserviceList{}
	}
	for _, broker := range instances.GetItems() {
		if broker.GetRemoteHost() == remoteHost {
			return broker
		}
	}

	newbroker := &Mqttservice{RemoteHost: remoteHost}
	instances.AddService(newbroker)
	return newbroker
}

func (p *Mqttservice) Startbroker(nodePort string, remoteUser string, logger logging.Logger) (string, error) {

	client, session, err := connectToHost(remoteUser, p.RemoteHost)
	if err != nil {
		logger.Debug("errore CONNESSIONE")
		logger.Debug(err.Error())
		return "", err
	}

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

	client, session, err = connectToHost(remoteUser, p.RemoteHost)
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

func (p *Mqttservice) GetRemoteHost() string {
	return p.RemoteHost
}

func Deletemqttservice(remoteHost string) {
	if instances == nil {
		return
	}
	for i, broker := range instances.GetItems() {
		if broker.GetRemoteHost() == remoteHost {
			remove(instances.items, i)
		}
	}
}
func remove(s []*Mqttservice, i int) []*Mqttservice {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func (p *Mqttservice) BrokerExist(remoteUser string, logger logging.Logger) bool {
	client, session, err := connectToHost(remoteUser, p.RemoteHost)
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
func (p *Mqttservice) Observebroker(nodePort string, remoteUser string, logger logging.Logger) (int, error) {

	logger.Debug("OSSERVO IL BROKER")
	resp, err := http.Get(fmt.Sprintf("http://%s:9399/metrics", p.RemoteHost))
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
			for _, tmp := range words {
				logger.Debug(tmp)
			}
			queue_str = words[1]
		}

	}
	queue, _ := strconv.ParseFloat(queue_str, 64)
	logger.Debug("LETTA LA DIMENSIONE DELLA CODA: ")
	logger.Debug(fmt.Sprintf("%d", int(queue)))
	return int(queue), nil

}

func (p *Mqttservice) Terminatebroker(nodePort string, remoteUser string, logger logging.Logger) error {

	client, session, err := connectToHost(remoteUser, p.RemoteHost)
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

func connectToHost(user, host string) (*ssh.Client, *ssh.Session, error) {

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.Password("dtazzioli")},
	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host+":22", sshConfig)
	if err != nil {
		return nil, nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	return client, session, nil
}
