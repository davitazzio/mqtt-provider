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
type mqttservice struct {
	RemoteHost string
}

type mqttserviceList struct {
	items []*mqttservice
}

func (pl *mqttserviceList) GetItems() []*mqttservice {
	return pl.items
}

func (pl *mqttserviceList) AddService(service *mqttservice) {
	pl.items = append(pl.items, service)

}

var instances *mqttserviceList

func GetInstance(remoteHost string) *mqttservice {

	// Pattern singleton for each brocker created
	if instances == nil {
		instances = &mqttserviceList{}
	}
	for _, broker := range instances.GetItems() {
		if broker.GetRemoteHost() == remoteHost {
			return broker
		}
	}

	newbroker := &mqttservice{RemoteHost: remoteHost}
	instances.AddService(newbroker)
	return newbroker
}

func (p *mqttservice) Startbroker(nodePort string, remoteUser string, logger logging.Logger) (string, error) {

	client, session, err := connectToHost(remoteUser, p.RemoteHost)
	if err != nil {
		logger.Debug("errore CONNESSIONE")
		logger.Debug(err.Error())
		return "", err
	}

	_, err = session.CombinedOutput(fmt.Sprintf("scp -r /home/dtazzioli/mqtt-provider/hivemq-4.28.0 %s@%s:/home/%s", remoteUser, p.RemoteHost, remoteUser))
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

func (p *mqttservice) GetRemoteHost() string {
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
func remove(s []*mqttservice, i int) []*mqttservice {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func (p *mqttservice) BrockerExist(remoteUser string, logger logging.Logger) bool {
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
	client.Close()
	return string(output) != ""
}
func (p *mqttservice) Observebroker(nodePort string, remoteUser string, logger logging.Logger) (int, error) {

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
		if words[0] == "com_hivemq_cluster_message_executor_queued_tasks" {
			fmt.Printf(words[1])
			queue_str = words[1]
		}

	}
	queue, _ := strconv.Atoi(queue_str)
	return queue, nil

}

func (p *mqttservice) Terminatebroker(nodePort string, remoteUser string, logger logging.Logger) error {

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

	client, err := ssh.Dial("tcp", host, sshConfig)
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
