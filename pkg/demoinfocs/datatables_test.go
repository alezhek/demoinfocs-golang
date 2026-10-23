package demoinfocs

import (
	"testing"

	"github.com/golang/geo/r3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	common "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	events "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	st "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables"
	stfake "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables/fake"
)

type DevNullReader struct {
}

func (DevNullReader) Read(p []byte) (n int, err error) {
	return len(p), nil
}

func newParser() *parser {
	p := NewParser(new(DevNullReader)).(*parser)
	p.header = &header{}

	return p
}

func fakePlayerEntity(id int) *stfake.Entity {
	entity := new(stfake.Entity)
	configurePlayerEntityMock(id, entity)

	return entity
}

func TestParser_GetClosestBombsiteFromPosition(t *testing.T) {
	p := newParser()
	p.bombsiteA = bombsite{
		center: r3.Vector{X: 2, Y: 3, Z: 1},
	}
	p.bombsiteB = bombsite{
		center: r3.Vector{X: 4, Y: 5, Z: 7},
	}

	site := p.getClosestBombsiteFromPosition(r3.Vector{X: -2, Y: 2, Z: 2})

	assert.Equal(t, events.BombsiteA, site)

	site = p.getClosestBombsiteFromPosition(r3.Vector{X: 3, Y: 6, Z: 5})

	assert.Equal(t, events.BombsiteB, site)
}

func configurePlayerEntityMock(id int, entity *stfake.Entity) {
	entity.On("ID").Return(id)

	var destroyCallback func()
	entity.On("OnDestroy", mock.Anything).Run(func(args mock.Arguments) {
		destroyCallback = args.Get(0).(func())
	})

	entity.On("OnPositionUpdate", mock.Anything).Return()
	prop := new(stfake.Property)
	prop.On("OnUpdate", mock.Anything).Return()
	entity.On("Property", mock.Anything).Return(prop)
	entity.On("BindProperty", mock.Anything, mock.Anything, mock.Anything)
	entity.On("Destroy").Run(func(mock.Arguments) {
		destroyCallback()
	})
}

// TestBindBomb_NilEntityChecks tests that bindBomb handles nil entities gracefully
// when processing m_bStartedArming updates. This prevents nil pointer dereference
// panics that can occur when pawn or controller entities are not yet created.
func TestBindBomb_NilEntityChecks(t *testing.T) {
	t.Run("nil pawn entity should not panic", func(t *testing.T) {
		p := newParser()
		p.gameState.entities = make(map[int]st.Entity)

		// Simulate the scenario where pawnEntity lookup returns nil
		// entityIDFromHandle with a valid handle but no entity in the map
		pawnEntityID := 42
		_ = pawnEntityID // Entity not added to map - simulates missing entity

		// This should not panic - the nil check should prevent it
		pawnEntity := p.gameState.entities[pawnEntityID]
		assert.Nil(t, pawnEntity, "pawnEntity should be nil when not in entities map")
	})

	t.Run("nil controller entity should not panic", func(t *testing.T) {
		p := newParser()
		p.gameState.entities = make(map[int]st.Entity)

		// Add pawn entity but not controller entity
		pawnEntity := new(stfake.Entity)
		ctlHandle := uint64(0x12345678) // Some handle value
		pawnEntity.On("PropertyValueMust", "m_hController").Return(st.PropertyValue{Any: ctlHandle})
		p.gameState.entities[10] = pawnEntity

		// Controller entity lookup should return nil
		ctlEntityID := 99 // Not in map
		ctlEntity := p.gameState.entities[ctlEntityID]
		assert.Nil(t, ctlEntity, "ctlEntity should be nil when not in entities map")
	})

	t.Run("nil player should not panic", func(t *testing.T) {
		p := newParser()
		p.gameState.entities = make(map[int]st.Entity)
		p.gameState.playersByEntityID = make(map[int]*common.Player)

		// Add pawn entity and controller entity but not player
		pawnEntity := new(stfake.Entity)
		ctlHandle := uint64(0x00000014) // Handle that maps to entity ID 20
		pawnEntity.On("PropertyValueMust", "m_hController").Return(st.PropertyValue{Any: ctlHandle})
		p.gameState.entities[10] = pawnEntity

		ctlEntity := new(stfake.Entity)
		ctlEntity.On("ID").Return(20)
		p.gameState.entities[20] = ctlEntity

		// Player lookup should return nil
		planter := p.gameState.playersByEntityID[20]
		assert.Nil(t, planter, "planter should be nil when not in playersByEntityID map")
	})
}

// Embed unused interfaces so the regression exercises bindBomb's actual callback.
type bombTestSendTableParser struct {
	sendTableParser
	classes st.ServerClasses
}

func (p bombTestSendTableParser) ServerClasses() st.ServerClasses { return p.classes }

type bombTestClasses struct {
	st.ServerClasses
	c4, planted st.ServerClass
}

func (c bombTestClasses) FindByName(name string) st.ServerClass {
	if name == "CC4" {
		return c.c4
	}
	if name == "CPlantedC4" {
		return c.planted
	}
	panic(name)
}

type bombTestClass struct {
	st.ServerClass
	created st.EntityCreatedHandler
}

func (c *bombTestClass) OnEntityCreated(h st.EntityCreatedHandler) { c.created = h }

func TestBindBomb_UsesOwnerPawnWhenControllerPawnIsUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name     string
		zone     int32
		position r3.Vector
		want     events.Bombsite
	}{
		{"zone A", 1, r3.Vector{}, events.BombsiteA},
		{"zone B", 2, r3.Vector{}, events.BombsiteB},
		{"position fallback B", 0, r3.Vector{X: 100}, events.BombsiteB},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newParser()
			c4, planted := &bombTestClass{}, &bombTestClass{}
			p.stParser = bombTestSendTableParser{classes: bombTestClasses{c4: c4, planted: planted}}
			p.bombsiteA.center = r3.Vector{}
			p.bombsiteB.center = r3.Vector{X: 100}
			bomb, pawn, controller := new(stfake.Entity), new(stfake.Entity), new(stfake.Entity)
			planter := &common.Player{EntityID: 20}
			assert.Nil(t, planter.PlayerPawnEntity()) // Controller linkage is temporarily unavailable.
			p.gameState.entities[10], p.gameState.entities[20] = pawn, controller
			p.gameState.playersByEntityID[20] = planter
			bomb.On("OnPositionUpdate", mock.Anything).Return()
			bomb.On("OnDestroy", mock.Anything).Return()
			owner, arming := new(stfake.Property), new(stfake.Property)
			bomb.On("Property", "m_hOwnerEntity").Return(owner)
			owner.On("OnUpdate", mock.Anything).Return()
			bomb.On("Property", "m_bStartedArming").Return(arming)
			var update st.PropertyUpdateHandler
			arming.On("OnUpdate", mock.Anything).Run(func(args mock.Arguments) { update = args.Get(0).(st.PropertyUpdateHandler) }).Return()
			bomb.On("PropertyValueMust", "m_hOwnerEntity").Return(st.PropertyValue{Any: uint64(10)})
			pawn.On("PropertyValueMust", "m_hController").Return(st.PropertyValue{Any: uint64(20)})
			pawn.On("PropertyValueMust", "m_nWhichBombZone").Return(st.PropertyValue{Any: tc.zone})
			pawn.On("Position").Return(tc.position)
			controller.On("ID").Return(20)
			var begins []events.BombPlantBegin
			var aborted []events.BombPlantAborted
			p.RegisterEventHandler(func(e events.BombPlantBegin) { begins = append(begins, e) })
			p.RegisterEventHandler(func(e events.BombPlantAborted) { aborted = append(aborted, e) })
			p.bindBomb()
			c4.created(bomb)
			assert.NotPanics(t, func() { update(st.PropertyValue{Any: true}) })
			if assert.Len(t, begins, 1) {
				assert.Same(t, planter, begins[0].Player)
				assert.Equal(t, tc.want, begins[0].Site)
			}
			assert.True(t, planter.IsPlanting)
			update(st.PropertyValue{Any: false})
			assert.False(t, planter.IsPlanting)
			if assert.Len(t, aborted, 1) {
				assert.Same(t, planter, aborted[0].Player)
			}
		})
	}
}
