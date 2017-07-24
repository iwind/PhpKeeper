package phpkeeper

import (
	"net"
	"bufio"
)

type Client struct {
	id int
	connection net.Conn
	whenClose func(client *Client)
	whenReceive func(client *Client, input string)
}

func (client *Client) handle() {
	input := bufio.NewScanner(client.connection)
	for input.Scan() {
		if client.whenReceive != nil {
			client.whenReceive(client, input.Text())
		}
	}

	defer func() {
		client.connection.Close()

		if client.whenClose != nil {
			client.whenClose(client)
		}
	}()
}

func (client *Client) write(message string) {
	client.connection.Write([]byte(message))
}