package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type P2PChat struct {
	peerConnection *webrtc.PeerConnection
	dataChannels   map[string]*webrtc.DataChannel
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	http.HandleFunc("/websocket", websocketHandler)
	go http.ListenAndServe(":8080", nil)

	fmt.Println("P2P Chat started. Enter 'offer' to create an offer, or paste an offer/answer to connect.")

	chat, err := newP2PChat()
	if err != nil {
		panic(err)
	}

	runChatLoop(chat)
}

func newP2PChat() (*P2PChat, error) {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, err
	}

	chat := &P2PChat{
		peerConnection: peerConnection,
		dataChannels:   make(map[string]*webrtc.DataChannel),
	}

	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i != nil {
			fmt.Println("New ICE Candidate:", i.ToJSON().Candidate)
		}
	})

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Println("New DataChannel:", d.Label())
		chat.dataChannels[d.Label()] = d
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))
		})
	})

	return chat, nil
}

func runChatLoop(chat *P2PChat) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()

		switch text {
		case "offer":
			handleOffer(chat, scanner)
		case "send":
			handleSend(chat, scanner)
		default:
			handleSDPExchange(chat, text)
		}
	}
}

func handleOffer(chat *P2PChat, scanner *bufio.Scanner) {
	dataChannel, err := chat.peerConnection.CreateDataChannel("chat", nil)
	if err != nil {
		fmt.Println("Error creating data channel:", err)
		return
	}
	chat.dataChannels[dataChannel.Label()] = dataChannel

	offer, err := createOffer(chat.peerConnection)
	if err != nil {
		fmt.Println("Error creating offer:", err)
		return
	}

	offerJSON, _ := json.Marshal(offer)
	encrypted, err := encrypt(string(offerJSON))
	if err != nil {
		fmt.Println("Error encrypting offer:", err)
		return
	}
	fmt.Println("Created offer. Send this to your peer:", encrypted)

	setupDataChannelHandlers(dataChannel)
}

func handleSend(chat *P2PChat, scanner *bufio.Scanner) {
	if len(chat.dataChannels) == 0 {
		fmt.Println("No active data channels. Establish a connection first.")
		return
	}

	fmt.Print("Enter message to send: ")
	scanner.Scan()
	message := scanner.Text()

	for label, dc := range chat.dataChannels {
		err := dc.SendText(message)
		if err != nil {
			fmt.Printf("Error sending message on channel %s: %v\n", label, err)
		} else {
			fmt.Printf("Message sent successfully on channel %s\n", label)
		}
	}
}

func handleSDPExchange(chat *P2PChat, text string) {
	decrypted, err := decrypt(text)
	if err != nil {
		fmt.Println("Error decrypting message:", err)
		return
	}

	var sd webrtc.SessionDescription
	err = json.Unmarshal([]byte(decrypted), &sd)
	if err != nil {
		fmt.Println("Invalid SDP:", err)
		return
	}

	err = chat.peerConnection.SetRemoteDescription(sd)
	if err != nil {
		fmt.Println("Error setting remote description:", err)
		return
	}

	if sd.Type == webrtc.SDPTypeOffer {
		handleIncomingOffer(chat)
	}
}

func handleIncomingOffer(chat *P2PChat) {
	answer, err := createAnswer(chat.peerConnection)
	if err != nil {
		fmt.Println("Error creating answer:", err)
		return
	}

	answerJSON, _ := json.Marshal(answer)
	encrypted, err := encrypt(string(answerJSON))
	if err != nil {
		fmt.Println("Error encrypting answer:", err)
		return
	}
	fmt.Println("Created answer. Send this to your peer:", encrypted)
}

func createOffer(peerConnection *webrtc.PeerConnection) (*webrtc.SessionDescription, error) {
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return nil, err
	}

	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		return nil, err
	}

	<-webrtc.GatheringCompletePromise(peerConnection)
	return peerConnection.LocalDescription(), nil
}

func createAnswer(peerConnection *webrtc.PeerConnection) (*webrtc.SessionDescription, error) {
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		return nil, err
	}

	<-webrtc.GatheringCompletePromise(peerConnection)
	return peerConnection.LocalDescription(), nil
}

func setupDataChannelHandlers(dataChannel *webrtc.DataChannel) {
	dataChannel.OnOpen(func() {
		fmt.Println("Data channel is open")
	})
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		fmt.Printf("Message from DataChannel: %s\n", string(msg.Data))
	})
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

func getSecretKey() []byte {
	key := os.Getenv("SECRET_KEY")
	if key == "" {
		key = "defaultsecretkey32byteslong12345"
	}
	return []byte(key)
}

func encrypt(jsonStr string) (string, error) {
	var jsonData interface{}
	err := json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		return "", fmt.Errorf("invalid JSON: %v", err)
	}

	compactJSON, err := json.Marshal(jsonData)
	if err != nil {
		return "", fmt.Errorf("error compacting JSON: %v", err)
	}

	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	_, err = w.Write(compactJSON)
	if err != nil {
		return "", fmt.Errorf("error compressing JSON: %v", err)
	}
	w.Close()

	block, err := aes.NewCipher(getSecretKey())
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, compressed.Bytes(), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decrypt(encryptedStr string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedStr)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(getSecretKey())
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]

	compressed, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	r, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return "", err
	}
	defer r.Close()

	var decompressed bytes.Buffer
	_, err = io.Copy(&decompressed, r)
	if err != nil {
		return "", err
	}

	return decompressed.String(), nil
}
