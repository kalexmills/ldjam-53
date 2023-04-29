package internal

import (
	"embed"
	"github.com/DrJosh9000/yarn"
	"github.com/DrJosh9000/yarn/bytecode"
	"github.com/Frabjous-Studios/ebitengine-game-template/internal/debug"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/razor-1/localizer-cldr/resources/language"
	"strings"
	"sync"
)

const yarnFile = "gamedata/yarn/bin/game"

// yarnBin contains all yarn output from this compilation process.
//
//go:embed gamedata/yarn/bin
var yarnBin embed.FS

type RunnerState uint8

const (
	RunnerStopped RunnerState = iota // RunnerStopped
	RunnerRunning                    // RunnerRunning is set for a runner that's running.
	RunnerWaiting                    // RunnerWaiting indicates the runner is waiting for the player to select an dialogueOption.
)

// DialogueRunner runs YarnSpinner and any commands from the script. It buffers lines delivered and handles blocking the
// YarnSpinner thread as expected by the game.
type DialogueRunner struct {
	program     *bytecode.Program
	stringTable *yarn.StringTable

	gameState    *GameState
	runState     RunnerState          // runState is manipulated by handler
	CurrNodeName string               // CurrNodeName is the name of the currently running node.
	vm           *yarn.VirtualMachine // vm is the Yarn virtual machine.

	mut *sync.RWMutex

	portraitImg *ebiten.Image
	portrait    Sprite

	running bool
}

func NewDialogueRunner(vars yarn.MapVariableStorage, handler yarn.DialogueHandler) (*DialogueRunner, error) {
	program, st, err := yarn.LoadFilesFS(yarnBin, yarnFile+".yarnc", language.EN_US)
	if err != nil {
		return nil, err
	}
	r := &DialogueRunner{
		program:     program,
		stringTable: st,
		runState:    RunnerStopped,
		mut:         &sync.RWMutex{},
		portraitImg: ebiten.NewImage(100, 100),
	}
	r.vm = &yarn.VirtualMachine{
		Program: r.program,
		Handler: handler,
		Vars:    vars,
	}
	return r, nil
}

// DoNode starts the runner, which blocks the current thread until a fatal error occurs.
func (r *DialogueRunner) DoNode(name string) error {
	defer func() {
		r.runState = RunnerStopped
	}()
	r.CurrNodeName = name
	r.portrait = nil
	r.runState = RunnerRunning

	return r.vm.Run(name)
}

func (r *DialogueRunner) Portrait() Sprite {
	if r.portrait != nil {
		return r.portrait
	}
	node, ok := r.vm.Program.Nodes[r.CurrNodeName]
	if !ok {
		debug.Printf("could not find node %v", r.CurrNodeName)
		return nil
	}
	r.portraitImg.Clear()
	portraitID := portrait(node)
	if portraitID == "random" {
		r.portrait = newRandPortrait(r.portraitImg)
		return r.portrait
	}
	toks := strings.Split(portraitID, ":")
	if len(toks) != 2 {
		debug.Printf("malformed portrait! using random: %v", portraitID)
		r.portrait = newRandPortrait(r.portraitImg)
		return r.portrait
	}
	head, body := toks[0], toks[1]
	r.portrait = newPortrait(r.portraitImg, body, head)
	return r.portrait
}

func (r *DialogueRunner) GameState() *GameState {
	r.gameState.Vars = r.vm.Vars.(yarn.MapVariableStorage)
	return r.gameState
}
func (r *DialogueRunner) IsLastLine(line yarn.Line) bool {
	if _, ok := r.stringTable.Table[line.ID]; !ok {
		return false
	}
	for _, tag := range r.stringTable.Table[line.ID].Tags {
		if tag == "lastline" {
			return true
		}
	}
	return false
}

func (r *DialogueRunner) Render(line yarn.Line) string {
	s, err := r.stringTable.Render(line)
	if err != nil {
		debug.Println("error rendering line", line)
		return "ERROR"
	}
	return s.String()
}

func portrait(node *bytecode.Node) string {
	if node == nil {
		return ""
	}
	for _, h := range node.Headers {
		if h.Key == "portrait" {
			return h.Value
		}
	}
	return ""
}
