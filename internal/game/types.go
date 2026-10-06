// Package game contains the authoritative, in-memory CookieKill simulation.
// A World must be protected by its caller's mutex. Snapshot and Profiles return
// independent values so callers may encode them after releasing that mutex.
package game

import "time"

type Input struct {
	X      float64 `json:"x"`
	Z      float64 `json:"z"`
	Yaw    float64 `json:"yaw"`
	Pitch  float64 `json:"pitch"`
	Sprint bool    `json:"sprint"`
	Jump   bool    `json:"jump"`
}

type Action struct {
	Action   string  `json:"action"`
	Item     string  `json:"item"`
	Target   string  `json:"target"`
	Amount   int     `json:"amount"`
	Slot     int     `json:"slot"`
	From     string  `json:"from"`
	To       string  `json:"to"`
	FromSlot int     `json:"fromSlot"`
	ToSlot   int     `json:"toSlot"`
	CallName string  `json:"callName"`
	Avatar   *Avatar `json:"avatar,omitempty"`
	Enabled  bool    `json:"enabled"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Z        float64 `json:"z"`
	Rotation float64 `json:"rotation"`
}

type Stack struct {
	Item  string `json:"item"`
	Count int    `json:"count"`
}
type Avatar struct {
	Skin       int    `json:"skin"`
	Shirt      string `json:"shirt"`
	Pants      string `json:"pants"`
	ShirtColor string `json:"shirtColor"`
	PantsColor string `json:"pantsColor"`
	Hat        string `json:"hat"`
}
type Home struct {
	Owner     string  `json:"owner"`
	CallName  string  `json:"callName"`
	Username  string  `json:"username"`
	Kind      string  `json:"kind"`
	X         float64 `json:"x"`
	Z         float64 `json:"z"`
	Safe      bool    `json:"safe"`
	Health    float64 `json:"health"`
	MaxHealth float64 `json:"maxHealth"`
}

// Inventory is a derived compatibility view; the three slot arrays own items.
type Profile struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Coins             int               `json:"coins"`
	Inventory         map[string]int    `json:"inventory"`
	Protected         []string          `json:"protected"`
	Levels            map[string]int    `json:"levels"`
	Bakery            bool              `json:"bakery"`
	BakeryLevel       int               `json:"bakeryLevel"`
	Reputation        map[string]int    `json:"reputation"`
	Kills             int               `json:"kills"`
	Deaths            int               `json:"deaths"`
	Selected          string            `json:"selected"`
	InventoryVersion  int               `json:"inventoryVersion"`
	SafeSlots         [3]Stack          `json:"safeSlots"`
	BagSlots          [15]Stack         `json:"bagSlots"`
	Hotbar            [5]Stack          `json:"hotbar"`
	SelectedSlot      int               `json:"selectedSlot"`
	Recovery          map[string]int    `json:"recovery"`
	Discovered        map[string]bool   `json:"discovered"`
	Username          string            `json:"username"`
	CallName          string            `json:"callName"`
	CallNameChangedAt int64             `json:"callNameChangedAt"`
	Avatar            Avatar            `json:"avatar"`
	Hats              map[string]bool   `json:"hats"`
	Cleaned           int               `json:"cleaned"`
	Rescues           int               `json:"rescues"`
	BakeryPlot        string            `json:"bakeryPlot"`
	Equipment         map[string]bool   `json:"equipment"`
	Paint             map[string]string `json:"paint"`
	Home              *Home             `json:"home,omitempty"`
	HomeIsland        *Island           `json:"homeIsland,omitempty"`
	FriendCode        string            `json:"friendCode"`
	PublicFriendCode  bool              `json:"publicFriendCode"`
	Friends           map[string]bool   `json:"friends"`
	FriendRequests    map[string]bool   `json:"friendRequests"`
	CamelOwned        bool              `json:"camelOwned"`
	CamelRentalUntil  int64             `json:"camelRentalUntil"`
	CamelDiscount     bool              `json:"camelDiscount"`
}

type PlayerView struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	X           float64            `json:"x"`
	Y           float64            `json:"y"`
	Z           float64            `json:"z"`
	Yaw         float64            `json:"yaw"`
	Pitch       float64            `json:"pitch"`
	Health      float64            `json:"health"`
	MaxHealth   float64            `json:"maxHealth"`
	Area        string             `json:"area"`
	Buffs       map[string]float64 `json:"buffs"`
	Username    string             `json:"username"`
	CallName    string             `json:"callName"`
	Avatar      Avatar             `json:"avatar"`
	Island      string             `json:"island"`
	FriendCode  string             `json:"friendCode,omitempty"`
	Posture     string             `json:"posture"`
	RidingCamel bool               `json:"ridingCamel"`
}

type Self struct {
	Profile
	X               float64            `json:"x"`
	Y               float64            `json:"y"`
	Z               float64            `json:"z"`
	Yaw             float64            `json:"yaw"`
	Pitch           float64            `json:"pitch"`
	Health          float64            `json:"health"`
	MaxHealth       float64            `json:"maxHealth"`
	Stamina         float64            `json:"stamina"`
	MaxStamina      float64            `json:"maxStamina"`
	Grounded        bool               `json:"grounded"`
	Sprinting       bool               `json:"sprinting"`
	SprintExhausted bool               `json:"sprintExhausted"`
	Area            string             `json:"area"`
	Buffs           map[string]float64 `json:"buffs"`
	Zone            string             `json:"zone"`
	CanBuildHome    bool               `json:"canBuildHome"`
	HomeSafeReason  string             `json:"homeSafeReason"`
	Island          string             `json:"island"`
	HomeOffer       bool               `json:"homeOffer"`
	FriendsTeleport bool               `json:"friendsTeleport"`
	Posture         string             `json:"posture"`
	RidingCamel     bool               `json:"ridingCamel"`
}

type Node struct {
	ID            string            `json:"id"`
	Kind          string            `json:"kind"`
	Label         string            `json:"label"`
	X             float64           `json:"x"`
	Z             float64           `json:"z"`
	Available     bool              `json:"available"`
	Amount        int               `json:"amount"`
	Owner         string            `json:"owner,omitempty"`
	CallName      string            `json:"callName,omitempty"`
	Username      string            `json:"username,omitempty"`
	Paint         map[string]string `json:"paint,omitempty"`
	Equipment     map[string]bool   `json:"equipment,omitempty"`
	BakeryLevel   int               `json:"bakeryLevel"`
	Island        string            `json:"island,omitempty"`
	ExpiresAt     float64           `json:"expiresAt,omitempty"`
	Scale         float64           `json:"scale,omitempty"`
	Variant       int               `json:"variant,omitempty"`
	Color         string            `json:"color,omitempty"`
	ChopRemaining int               `json:"chopRemaining,omitempty"`
	ChopTotal     int               `json:"chopTotal,omitempty"`
	readyAt       float64
	respawn       float64
}

type Animal struct {
	ID                                                    string  `json:"id"`
	Species                                               string  `json:"species"`
	X                                                     float64 `json:"x"`
	Z                                                     float64 `json:"z"`
	Health                                                float64 `json:"health"`
	MaxHealth                                             float64 `json:"maxHealth"`
	Need                                                  string  `json:"need"`
	Disposition                                           string  `json:"disposition"`
	Group                                                 string  `json:"group"`
	Speed                                                 float64 `json:"speed"`
	Yaw                                                   float64 `json:"yaw"`
	Y                                                     float64 `json:"y"`
	Heading                                               float64 `json:"heading"`
	Scale                                                 float64 `json:"scale"`
	GroupID                                               string  `json:"groupId"`
	Egg                                                   bool    `json:"egg"`
	HatchIn                                               float64 `json:"hatchIn"`
	Age                                                   float64 `json:"age"`
	lifespan, sickUntil, eggSince, jumpPhase, wanderPhase float64
	groupX, groupZ, orbitRadius, baseSpeed, injuryRate    float64
	homeX                                                 float64
	homeZ                                                 float64
	phase                                                 float64
	attackAt                                              float64
	respawnAt                                             float64
	needAt                                                float64
	rewardAt                                              map[string]float64
}

type Projectile struct {
	ID     string  `json:"id"`
	Owner  string  `json:"owner"`
	Item   string  `json:"item"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
	VX     float64 `json:"vx"`
	VY     float64 `json:"vy"`
	VZ     float64 `json:"vz"`
	damage float64
	born   float64
	super  bool
}

type Event struct {
	ID      uint64  `json:"id"`
	Actor   string  `json:"actor,omitempty"`
	Text    string  `json:"text"`
	Kind    string  `json:"kind"`
	X       float64 `json:"x"`
	Z       float64 `json:"z"`
	Time    float64 `json:"time"`
	HitZone string  `json:"hitZone,omitempty"`
	Reward  int     `json:"reward,omitempty"`
}

type Recipe struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Damage      float64        `json:"damage"`
	Heal        float64        `json:"heal"`
	Cost        map[string]int `json:"cost"`
	Known       bool           `json:"known"`
}

type Snapshot struct {
	Time         float64       `json:"time"`
	ServerTime   int64         `json:"serverTime"`
	Me           Self          `json:"me"`
	Players      []PlayerView  `json:"players"`
	Projectiles  []Projectile  `json:"projectiles"`
	Nodes        []Node        `json:"nodes"`
	Animals      []Animal      `json:"animals"`
	Events       []Event       `json:"events"`
	Recipes      []Recipe      `json:"recipes"`
	Homes        []Home        `json:"homes"`
	Layout       *Layout       `json:"layout,omitempty"`
	Island       *Island       `json:"island,omitempty"`
	BuildCatalog []BuildRecipe `json:"buildCatalog"`
	Friends      []FriendView  `json:"friends"`
}

type player struct {
	profile                                 Profile
	island, posture                         string
	y                                       float64
	jumpOffset, jumpVelocity                float64
	jumpQueued, sprinting, sprintExhausted  bool
	jumpHeld, sprintHeld                    bool
	homeOffer, friendsTeleport, ridingCamel bool
	x, z, yaw, pitch                        float64
	health, stamina                         float64
	input                                   Input
	buffs                                   map[string]float64
	cooldowns                               map[string]float64
}

type World struct {
	time              float64
	sequence          uint64
	players           map[string]*player
	offline           map[string]*player
	profiles          map[string]Profile
	nodes             []*Node
	animals           []*Animal
	projectiles       []*Projectile
	events            []Event
	layout            Layout
	colliders         []Collider
	propertyColliders []Collider
	now               func() time.Time
}
