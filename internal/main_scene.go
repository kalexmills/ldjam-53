package internal

import (
	"fmt"
	"github.com/DrJosh9000/yarn"
	"github.com/Frabjous-Studios/ebitengine-game-template/internal/debug"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"image"
	"log"
	"strings"
	"sync"
)

// global ScaleFactor for pixel art.
const ScaleFactor = 2.0

type BaseScene struct {
}

type Sprite interface {
	DrawTo(screen *ebiten.Image)
	Bounds() image.Rectangle
	Pos() image.Point
	SetPos(image.Point)
}

type BaseSprite struct {
	X, Y int
	Img  *ebiten.Image
}

func (s *BaseSprite) DrawTo(screen *ebiten.Image) {
	opt := &ebiten.DrawImageOptions{}
	opt.GeoM.Translate(float64(s.X), float64(s.Y))
	opt.GeoM.Scale(ScaleFactor, ScaleFactor)
	screen.DrawImage(s.Img, opt)
}

func (s *BaseSprite) Bounds() image.Rectangle {
	return s.Img.Bounds().Add(image.Pt(s.X, s.Y))
}

func (s *BaseSprite) Pos() image.Point {
	return image.Pt(s.X, s.Y)
}

func (s *BaseSprite) SetPos(pt image.Point) {
	s.X = pt.X
	s.Y = pt.Y
}

func (s *BaseSprite) ClampToRect(r image.Rectangle) {
	s.X = clamp(s.X, r.Min.X, r.Max.X-s.Bounds().Dx())
	s.Y = clamp(s.Y, r.Min.Y, r.Max.Y-s.Bounds().Dy())
}

type MainScene struct {
	Game *Game

	Day      Day // Customers is a list of Yarnspinner nodes happening on the current day
	Customer Sprite

	Sprites []Sprite

	State  *GameState
	Runner *DialogueRunner

	till    *Till
	counter *BaseSprite

	bubbles *Bubbles

	holding     Sprite
	clickStart  image.Point
	clickOffset image.Point

	vars yarn.MapVariableStorage
	mut  sync.Mutex

	wait *sync.Cond
}

func NewMainScene(g *Game) *MainScene {
	runner, err := NewDialogueRunner()
	if err != nil {
		log.Fatal(err)
	}
	result := &MainScene{
		Game:   g,
		Runner: runner,
		Sprites: []Sprite{
			newBill(1, 5, 60),
			newBill(5, 60, 60),
			newBill(10, 45, 60),
			newCoin(1, 5, 20),
			newCoin(5, 25, 20),
			newCoin(25, 45, 20),
		},
		Day: Days[0],
		State: &GameState{
			CurrentNode: "Start",
			Vars:        make(yarn.MapVariableStorage),
		},

		till: NewTill(),
		counter: &BaseSprite{
			X: 112, Y: 152,
			Img: Resources.images["counter"],
		},
		vars: make(yarn.MapVariableStorage),
	}
	result.bubbles = NewBubbles(result)
	result.wait = sync.NewCond(&result.mut)
	return result
}

func (m *MainScene) Update() error {
	if !m.Runner.running {
		go m.startRunner()
		m.Runner.running = true
	}
	m.bubbles.Update()

	cPos := cursorPos()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if m.holding != nil {
			fmt.Println("tillBounds", m.till.Bounds(), "cPos", cPos)
			if cPos.In(m.till.Bounds()) { // if over Till; drop on Till
				fmt.Println("inside!")
				if !m.till.Drop(m.holding) {
					m.counterDrop()
				} else {
					m.holding = nil
				}
			} else {
				m.counterDrop()
			}
		} else { // pick up the thing under the cursor
			m.holding = m.spriteUnderCursor()
			if m.holding != nil {
				m.clickStart = cPos
				m.clickOffset = m.holding.Pos().Sub(m.clickStart)
				m.till.Remove(m.holding) // remove it from the Till (maybe)
			}
		}
	}

	if m.holding != nil {
		mPos := cursorPos()
		m.holding.SetPos(mPos.Add(m.clickOffset))
	}

	return nil
}

func (m *MainScene) Draw(screen *ebiten.Image) {
	// draw Till
	m.till.DrawTo(screen)

	// draw counter
	m.counter.DrawTo(screen)

	// draw all the sprites in their draw order.
	for _, sprite := range m.Sprites {
		sprite.DrawTo(screen)
	}

	// draw text
	m.bubbles.DrawTo(screen)
}

type DialogueLine struct {
	*BaseSprite // 9-patch

	Line        string
	IsCustomer  bool
	Highlighted bool
	Clickbox    image.Rectangle
}

func (m *MainScene) startRunner() {
	if err := m.Runner.Start(m.vars, m); err != nil {
		debug.Printf("error starting runner: %v", err)
		return
	}
}

func (m *MainScene) pickUp() {
	m.holding = m.spriteUnderCursor()
	if m.holding != nil {
		m.clickStart = cursorPos()
		m.clickOffset = m.holding.Pos().Sub(m.clickStart)
	}
}

func (m *MainScene) counterDrop() {
	m.holding.SetPos(clampToCounter(m.holding.Pos()))
	m.holding = nil
}

func (m *MainScene) spriteUnderCursor() Sprite {
	for _, sprite := range m.Sprites {
		if cursorPos().In(sprite.Bounds()) {
			return sprite
		}
	}
	return nil
}

func (m *MainScene) NodeStart(name string) error {
	fmt.Println("start node", name)
	return nil
}

func (m *MainScene) PrepareForLines(lineIDs []string) error {
	return nil
}

func (m *MainScene) Line(line yarn.Line) error {
	m.mut.Lock()
	defer m.mut.Unlock()
	m.bubbles.SetLine(m.Runner.Render(line))
	for !m.bubbles.IsDone() {
		fmt.Println("locked!")
		m.wait.Wait()
		fmt.Println("unlocked; checking again")
	}
	fmt.Println("done with line!")
	return nil
}

func (m *MainScene) Options(options []yarn.Option) (int, error) {
	fmt.Println("options", options)
	return 0, nil
}

func (m *MainScene) Command(command string) error {
	fmt.Println("run command:", command)
	command = strings.TrimSpace(command)
	tokens := strings.Split(command, " ")
	if len(tokens) == 0 {
		return fmt.Errorf("bad command: %s", command)
	}
	switch tokens[0] {
	default:
		return fmt.Errorf("unknown command %s", tokens[0])
	}
}

func (m *MainScene) NodeComplete(nodeName string) error {
	fmt.Println("node done", nodeName)
	return nil
}

func (m *MainScene) DialogueComplete() error {
	fmt.Println("dialogue complete")
	return nil
}

type Portrait struct {
	*BaseSprite
}

func newPortrait(body, head string) Sprite {
	b, h := Resources.bodies[body], Resources.heads[head]
	img := ebiten.NewImage(100, 100)
	img.DrawImage(b, nil)
	img.DrawImage(h, nil)
	return &Portrait{
		BaseSprite: &BaseSprite{
			Img: img,
			X:   170,
			Y:   52,
		},
	}
}

func newRandPortrait() Sprite {
	return newPortrait(randMapKey(Resources.bodies), randMapKey(Resources.heads))
}

// clampToCounter clamps the provided point to the counter range (hardcoded)
func clampToCounter(pt image.Point) image.Point {
	pt.X = clamp(pt.X, 112, 320-15)
	pt.Y = clamp(pt.Y, 152, 240-15)
	return pt
}

func clamp(x int, min, max int) int {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}

func cursorPos() image.Point {
	mx, my := ebiten.CursorPosition()
	return image.Pt(mx/2, my/2)
}

func rect(x, y, w, h int) image.Rectangle {
	return image.Rect(x, y, x+w, y+h)
}
