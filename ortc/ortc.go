package ortc

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"strings"

	"github.com/pion/webrtc/v3"
)

var stunServers = []string{
	"stun:stun.l.google.com:19302",
}

type ORTC struct {
	api *webrtc.API

	gatherer  *webrtc.ICEGatherer
	transport *webrtc.ICETransport
	dtls      *webrtc.DTLSTransport
	sctp      *webrtc.SCTPTransport
	channel   *webrtc.DataChannel

	localToken token

	onMessage func([]byte)
}

func NewORTC() *ORTC {
	return &ORTC{
		api: webrtc.NewAPI(),
	}
}

type token struct {
	ICECandidates    []webrtc.ICECandidate
	ICEParameters    webrtc.ICEParameters
	DTLSParameters   webrtc.DTLSParameters
	SCTPCapabilities webrtc.SCTPCapabilities
}

func (o *ORTC) LocalToken() (localTokenStr string, err error) {
	o.gatherer, err = o.api.NewICEGatherer(webrtc.ICEGatherOptions{
		ICEServers: []webrtc.ICEServer{{URLs: stunServers}},
	})
	if err != nil {
		return "", err
	}
	o.transport = o.api.NewICETransport(o.gatherer)
	o.dtls, err = o.api.NewDTLSTransport(o.transport, nil)
	if err != nil {
		return "", err
	}
	o.sctp = o.api.NewSCTPTransport(o.dtls)

	done := make(chan struct{})
	o.gatherer.OnLocalCandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			close(done)
			return
		}
		log.Println("Found candidate:", c)
	})
	err = o.gatherer.Gather()
	if err != nil {
		return "", err
	}
	<-done

	o.localToken.ICECandidates, err = o.gatherer.GetLocalCandidates()
	if err != nil {
		return "", err
	}
	o.localToken.ICEParameters, err = o.gatherer.GetLocalParameters()
	if err != nil {
		return "", err
	}
	o.localToken.DTLSParameters, err = o.dtls.GetLocalParameters()
	if err != nil {
		return "", err
	}
	o.localToken.SCTPCapabilities = o.sctp.GetCapabilities()
	if err != nil {
		return "", err
	}

	localTokenStrBuilder := strings.Builder{}
	enc := json.NewEncoder(base64.NewEncoder(base64.StdEncoding, &localTokenStrBuilder))
	err = enc.Encode(o.localToken)
	if err != nil {
		return "", err
	}

	return localTokenStrBuilder.String(), nil
}

func (o *ORTC) Start(remoteTokenStr string) (err error) {
	dec := json.NewDecoder(base64.NewDecoder(base64.StdEncoding, strings.NewReader(remoteTokenStr)))
	remoteToken := token{}
	err = dec.Decode(&remoteToken)
	if err != nil {
		return err
	}

	err = o.transport.SetRemoteCandidates(remoteToken.ICECandidates)
	if err != nil {
		return err
	}
	role := webrtc.ICERoleControlled
	if o.localToken.ICEParameters.UsernameFragment > remoteToken.ICEParameters.UsernameFragment {
		role = webrtc.ICERoleControlling
	}
	err = o.transport.Start(o.gatherer, remoteToken.ICEParameters, &role)
	if err != nil {
		return err
	}
	err = o.dtls.Start(remoteToken.DTLSParameters)
	if err != nil {
		return err
	}

	o.sctp.OnDataChannelOpened(func(channel *webrtc.DataChannel) {
		channel.OnMessage(func(msg webrtc.DataChannelMessage) {
			if o.onMessage == nil {
				return
			}
			o.onMessage(msg.Data)
		})
	})
	err = o.sctp.Start(remoteToken.SCTPCapabilities)
	if err != nil {
		return err
	}
	o.channel, err = o.api.NewDataChannel(o.sctp, &webrtc.DataChannelParameters{})
	if err != nil {
		return err
	}

	return nil
}

func (o *ORTC) OnMessage(f func(msg []byte)) {
	o.onMessage = f
}

func (o *ORTC) SendMessage(msg []byte) error {
	return o.channel.Send(msg)
}
