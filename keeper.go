package phpkeeper

import (
	"net"
	"os"
	"log"
	"time"
	"sync"
	"math/rand"
	"encoding/json"
	"io/ioutil"
)

type Keeper struct {
	taskServer net.Listener
	taskClients map[int] Client

	processServer net.Listener
	processClients []Client
	processes []os.Process

	mu sync.Mutex
}

var AppDir string
var config = Config{}

func Start(appDir string) {
	AppDir = appDir

	keeper := Keeper{}
	keeper.Start()
}

func (keeper *Keeper)Start()  {
	//设置日志
	logFile, err := os.OpenFile(AppDir + string(os.PathSeparator) + "run.log", os.O_APPEND | os.O_WRONLY | os.O_CREATE, 0777)
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	//加载配置
	configFile := AppDir + string(os.PathSeparator) + "config.json"
	bytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Println(err)
		return
	}
	err = json.Unmarshal(bytes, &config)

	if err != nil {
		log.Println(err)
		return
	}

	if config.Count == 0 {
		return
	}

	//启动服务
	go keeper.startTaskServer()
	go keeper.startProcessServer()
	go keeper.startProcesses()

	//主进程等待
	for {
		time.Sleep(time.Hour)
	}
}

func (keeper *Keeper) startTaskServer() {
	server, err := net.Listen("tcp", config.Server.Task)
	if err != nil {
		log.Println(err)
		return
	}

	keeper.taskServer = server
	keeper.taskClients = map[int]Client{}

	//接受链接
	var id int = 0
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println("accept new client")
		id ++

		client := Client{
			id: id,
			connection: conn,
			whenClose: func(client *Client) {
				keeper.mu.Lock()
				delete(keeper.taskClients, client.id)
				keeper.mu.Unlock()

				log.Println("close task", client.id, "left:", len(keeper.processClients))
			},
			whenReceive: func(client *Client, input string) {
				log.Println("comming task", client.id)

				countProcessClients := len(keeper.processClients)
				if countProcessClients > 0 {
					rand.Seed(time.Now().UnixNano())
					processClientIndex := rand.Int() % countProcessClients

					log.Println("process task", processClientIndex)

					//写入到process clients中
					keeper.processClients[processClientIndex].write(input + "\n")

					client.write("success\n")
				} else {
					client.write("fail\n")
				}
			},
		}
		keeper.taskClients[id] = client

		go client.handle()
	}
}

func (keeper *Keeper) startProcesses() {
	if config.Count <= 0 {
		return
	}

	envVars := []string{}
	for key, value := range config.Env {
		envVars = append(envVars, key + "=" + value)
	}

	for i := 0; i < config.Count; i ++ {
		go func () {
			for {
				attrs := os.ProcAttr{
					Dir: config.Dir,
					Env: envVars,
					Files:[]*os.File{ os.Stdin, os.Stdout, os.Stderr },
				}
				log.Println(config.Process, config.Args)
				process, err := os.StartProcess(config.Process, config.Args, &attrs)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println(process.Pid)
				process.Wait()
				log.Println("end")
			}
		}()
	}
}

func (keeper *Keeper) startProcessServer() {
	server, err := net.Listen("tcp", config.Server.Process)
	if err != nil {
		log.Println(err)
		return
	}

	keeper.processServer = server
	keeper.processClients = []Client{}

	//接受链接
	var id int = 0
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println("accept new client")
		id ++

		client := Client{
			id: id,
			connection: conn,
			whenClose: func(client *Client) {
				keeper.mu.Lock()

				for index, clientItem := range keeper.processClients {
					if clientItem.id == client.id {
						keeper.processClients = append(keeper.processClients[:index], keeper.processClients[index+1:]...)
						break
					}
				}

				keeper.mu.Unlock()
				log.Println("close process", client.id, "left:", keeper.processClients)
			},
			whenReceive: func(client *Client, input string) {
				//log.Println("process", client.id, input)
			},
		}
		keeper.processClients = append(keeper.processClients, client)

		go client.handle()
	}
}
