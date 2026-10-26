package lxphost

type Code int32

func (c Code) Error() string {
	if name, ok := codeNames[c]; ok {
		return name
	}
	return "LXP_ERR_UNKNOWN"
}
