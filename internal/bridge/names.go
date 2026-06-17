package bridge

type nameMapper struct {
}

func newNameMapper(names []string) nameMapper {
	return nameMapper{}
}

func (m nameMapper) exposed(serverName, original string) string {
	return serverName + "__" + original
}
