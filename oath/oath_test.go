package oath

import (
	"fmt"
	"testing"
)

/*
[yubo@yubo-990:~/bin][master]$date "+%s"
1446108404
[yubo@yubo-990:~/bin][master]$oathtool --totp -b 7KZZ4VRQBX2SA6E5 -w 4
234977
002761
769949
745702
560874
*/

func Test_otp(t *testing.T) {
	secret := "7KZZ4VRQBX2SA6E5"
	time := int64(1446108404)

	fmt.Println(Oaths(secret, 4))
	fmt.Println(OathOtp(secret, true, 6,
		time, 30, 4))
}
