package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

var activeDataChannel *webrtc.DataChannel

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	http.HandleFunc("/websocket", websocketHandler)
	go http.ListenAndServe(":8080", nil)

	fmt.Println("P2P Chat started. Enter 'offer' to create an offer, or paste an offer/answer to connect.")
	
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}

	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i != nil {
			fmt.Println("New ICE Candidate:", i.ToJSON().Candidate)
		}
	})

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Println("New DataChannel:", d.Label())
    activeDataChannel = d
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))
		})
	})

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()

		if text == "offer" {
			// Create a data channel
			dataChannel, err := peerConnection.CreateDataChannel("chat", nil)
			if err != nil {
				panic(err)
			}

			// Create an offer
			offer, err := peerConnection.CreateOffer(nil)
			if err != nil {
				panic(err)
			}
      activeDataChannel = dataChannel

			// Set the local description
			err = peerConnection.SetLocalDescription(offer)
			if err != nil {
				panic(err)
			}

			// Wait for ICE gathering to complete
			<-webrtc.GatheringCompletePromise(peerConnection)

			// Get the local description with ICE candidates
			localDesc := peerConnection.LocalDescription()

			// Marshal the offer to JSON
			offerJSON, _ := json.Marshal(localDesc)
			fmt.Println("Created offer. Send this to your peer:", string(offerJSON))

			// Set up the data channel handlers
			dataChannel.OnOpen(func() {
				fmt.Println("Data channel is open")
			})
			dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
				fmt.Printf("Message from DataChannel: %s\n", string(msg.Data))
			})
		} else if text == "send" {
      if activeDataChannel == nil {
        fmt.Println("No active data channel. Establish a connection first.")
        continue
      }
      fmt.Print("Enter message to send: ")
      scanner.Scan()
      message := scanner.Text()
      err := activeDataChannel.SendText(message)
      if err != nil {
        fmt.Println("Error sending message:", err)
      } else {
        fmt.Println("Message sent successfully")
      } 
    } else {
			var sd webrtc.SessionDescription
			err := json.Unmarshal([]byte(text), &sd)
			if err != nil {
				fmt.Println("Invalid SDP:", err)
				continue
			}

			if sd.Type == webrtc.SDPTypeOffer {
				err = peerConnection.SetRemoteDescription(sd)
				if err != nil {
					panic(err)
				}

				// Create an answer
				answer, err := peerConnection.CreateAnswer(nil)
				if err != nil {
					panic(err)
				}

				// Set the local description
				err = peerConnection.SetLocalDescription(answer)
				if err != nil {
					panic(err)
				}

				// Wait for ICE gathering to complete
				<-webrtc.GatheringCompletePromise(peerConnection)

				// Get the local description with ICE candidates
				localDesc := peerConnection.LocalDescription()

				// Marshal the answer to JSON
				answerJSON, _ := json.Marshal(localDesc)
				fmt.Println("Created answer. Send this to your peer:", string(answerJSON))
			} else if sd.Type == webrtc.SDPTypeAnswer {
				err = peerConnection.SetRemoteDescription(sd)
				if err != nil {
					panic(err)
				}
			}
		}
	}
}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading to WebSocket:", err)
		return
	}
	defer conn.Close()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err := conn.WriteMessage(messageType, p); err != nil {
			return
		}
	}
}
