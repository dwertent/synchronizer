package domain

// ObjectDeleted represents a ObjectDeleted model.
type ObjectDeleted struct {
	Account              string
	Cluster              string
	Depth                int
	Event                *Event
	Kind                 *Kind
	MsgId                string
	Name                 string
	AdditionalProperties map[string]interface{}
}
