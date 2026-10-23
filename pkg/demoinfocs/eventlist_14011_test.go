package demoinfocs

import (
	"testing"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestProtocol14011FallbackSoundAndPing(t *testing.T) {
	b, err := getGameEventListBinForProtocol(14011)
	require.NoError(t, err)
	list := new(msg.CMsgSource1LegacyGameEventList)
	require.NoError(t, proto.Unmarshal(b, list))
	p := newParser()
	p.handleGameEventList(list)
	d := p.gameEventDescs[266]
	require.NotNil(t, d)
	require.Equal(t, "player_sound", d.GetName())
	names := []string{}
	for _, k := range d.Keys {
		names = append(names, k.GetName())
	}
	assert.Equal(t, []string{"userid", "userid_pawn", "radius", "duration", "step"}, names)
	event := &msg.CMsgSource1LegacyGameEvent{Eventid: proto.Int32(266), Keys: []*msg.CMsgSource1LegacyGameEventKeyT{
		{Type: proto.Int32(9), ValShort: proto.Int32(6)},
		{Type: proto.Int32(8), ValLong: proto.Int32(-1570143984)},
		{Type: proto.Int32(3), ValLong: proto.Int32(600)},
		{Type: proto.Int32(2), ValFloat: proto.Float32(0.1)},
		{Type: proto.Int32(6), ValBool: proto.Bool(false)},
	}}
	data := mapGameEventData(d, event)
	assert.Equal(t, int32(6), data["userid"].GetValShort())
	assert.Equal(t, int32(600), data["radius"].GetValLong())
	require.Equal(t, "player_ping_stop", p.gameEventDescs[265].GetName())
	assert.Len(t, p.gameEventDescs[265].Keys, 1)
}

func TestProtocol14011FallbackDoesNotChangeOtherProtocols(t *testing.T) {
	for _, protocol := range []int{13992, 14010, 14012, 14020} {
		got, err := getGameEventListBinForProtocol(protocol)
		require.NoError(t, err)
		want, err := eventListFolder.ReadFile("event-list-dump/13992.bin")
		require.NoError(t, err)
		assert.Equal(t, want, got, "protocol %d", protocol)
	}
}

func TestProtocol14021UsesSameVerifiedEventList(t *testing.T) {
	a, err := getGameEventListBinForProtocol(14011)
	require.NoError(t, err)
	b, err := getGameEventListBinForProtocol(14021)
	require.NoError(t, err)
	assert.Equal(t, a, b)
}

func TestProtocol14022UsesSameVerifiedEventList(t *testing.T) {
	a, err := getGameEventListBinForProtocol(14011)
	require.NoError(t, err)
	b, err := getGameEventListBinForProtocol(14022)
	require.NoError(t, err)
	assert.Equal(t, a, b)
	c, err := getGameEventListBinForProtocol(14023)
	require.NoError(t, err)
	want, err := eventListFolder.ReadFile("event-list-dump/14023.bin")
	require.NoError(t, err)
	assert.Equal(t, want, c)
}
