package demoinfocs

import (
	"crypto/rand"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/constants"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
	st "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables"
	stfake "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables/fake"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestPlayerHurtDamagePawnGeneration(t *testing.T) {
	for _, tc := range []struct {
		name                                     string
		serial                                   int
		missingPawn, missingHealth, missingArmor bool
		wantError                                bool
	}{
		{name: "valid lethal hit", serial: 485},
		{name: "recycled index", serial: 487, wantError: true},
		{name: "pawn absent", missingPawn: true, wantError: true},
		{name: "health absent", serial: 485, missingHealth: true, wantError: true},
		{name: "armor absent", serial: 485, missingArmor: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser(rand.Reader).(*parser)
			defer p.Close()
			p.gameState.ingameTick = 34928
			handle := uint64(485<<constants.MaxEdictBitsSource2 | 445)
			ctl := stfake.NewEntityWithProperty("m_hPlayerPawn", st.PropertyValue{Any: handle})
			ctl.On("PropertyValue", "m_hPawn").Return(st.PropertyValue{Any: handle}, true)
			pl := common.NewPlayer(p.demoInfoProvider)
			pl.Entity = ctl
			pl.UserID = 7
			p.gameState.playersByUserID[7] = pl
			if !tc.missingPawn {
				pawn := new(stfake.Entity)
				pawn.On("ID").Return(445)
				pawn.On("SerialNum").Return(tc.serial)
				var hp, armor st.Property
				if !tc.missingHealth {
					hp = new(stfake.Property)
				}
				if !tc.missingArmor {
					armor = new(stfake.Property)
				}
				pawn.On("Property", "m_iHealth").Return(hp)
				pawn.On("Property", "m_ArmorValue").Return(armor)
				pawn.On("PropertyValueMust", "m_iHealth").Return(st.PropertyValue{Any: int32(5)})
				pawn.On("PropertyValueMust", "m_ArmorValue").Return(st.PropertyValue{Any: int32(2)})
				p.gameState.entities[445] = pawn
			}
			count := 0
			p.RegisterEventHandler(func(e events.PlayerHurt) {
				count++
				require.Equal(t, 5, e.HealthDamageTaken)
				require.Equal(t, 2, e.ArmorDamageTaken)
				require.Equal(t, 15, e.HealthDamage)
			})
			data := map[string]*msg.CMsgSource1LegacyGameEventKeyT{
				"userid": {ValShort: proto.Int32(7)}, "attacker": {ValShort: proto.Int32(7)},
				"health": {ValByte: proto.Int32(0)}, "armor": {ValByte: proto.Int32(0)},
				"dmg_health": {ValShort: proto.Int32(15)}, "dmg_armor": {ValByte: proto.Int32(10)},
				"weapon": {ValString: proto.String("knife")},
			}
			require.NotPanics(t, func() { p.gameEventHandler.playerHurt(data) })
			if tc.wantError {
				require.ErrorIs(t, p.error(), ErrPlayerDamageStateUnavailable)
				require.Equal(t, 0, count)
			} else {
				require.NoError(t, p.error())
				require.Equal(t, 1, count)
			}
		})
	}
}
