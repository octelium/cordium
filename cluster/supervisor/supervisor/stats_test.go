/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License version 3,
 * as published by the Free Software Foundation of the License.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package supervisor

/*
import (
	"encoding/json"
	"testing"

	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/stretchr/testify/assert"
)

func TestParseContainerStats(t *testing.T) {
	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	example := `
 {"id":"d0ee64bd5b34","name":"workspace","cpu_time":"5m31.298391s","cpu_percent":"43.56%","avg_cpu":"43.56%","mem_usage":"494.4MB / 4.001GB","mem_percent":"12.36%","net_io":"1.056MB / 416.8MB","block_io":"693.9MB / 3.662GB","pids":"207"}
 `

	stat := &containerStats{}
	err = json.Unmarshal([]byte(example), stat)
	assert.Nil(t, err)

	out := parseContainerStats(stat)
	assert.NotNil(t, out)
	assert.Equal(t, float32(43.56), out.Cpu.Percent)
	assert.Equal(t, int64(207), out.TotalPIDs)
	assert.Equal(t, int64(494.4*1000*1000), out.Memory.CurBytes)
	assert.Equal(t, int64(4.001*1000*1000*1000), out.Memory.TotalBytes)
}

func TestPercentToFloat(t *testing.T) {
	assert.Equal(t, float32(33.33), percentToFloat("33.33%"))
}

func TestParseBytes(t *testing.T) {
	assert.Equal(t, int64(3*1000*1000), parseBytes("3MB"))
}

func TestParsePercentage(t *testing.T) {
	o1, o2 := parsePercentage("494.4MB / 4.001GB")
	assert.Equal(t, "494.4MB", o1)
	assert.Equal(t, "4.001GB", o2)
}
*/
