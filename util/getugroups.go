package util

func GetUGroups(username string) ([]uint32, error) {
	return getugroups(username)
}
