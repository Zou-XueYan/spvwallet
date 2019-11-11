package alliance

type Checkpoint struct {
	Height uint32
}

var alliaCheckPoints map[string]*Checkpoint

func init() {
	alliaCheckPoints = make(map[string]*Checkpoint)

	alliaCheckPoints["testnet"] = &Checkpoint{
		Height: 1,
	}
	alliaCheckPoints["regtest"] = &Checkpoint{
		Height: 1,
	}
}
