/*******************************************************************************
 * Copyright (c) 2024 Genome Research Ltd.
 *
 * Permission is hereby granted, free of charge, to any person obtaining
 * a copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 *
 * The above copyright notice and this permission notice shall be included
 * in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
 * CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
 * TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 ******************************************************************************/

package debounce

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDebounce(t *testing.T) {
	Convey("Given a Debounce op", t, func() {
		startTimes := make(chan time.Time, 2)
		endTimes := make(chan time.Time, 2)
		callLength := 150 * time.Millisecond
		counter := 0
		throttleOp := func() {
			startTimes <- time.Now()

			<-time.After(callLength)

			counter += 1

			endTimes <- time.Now()
		}

		Convey("Debounce runs long-running tasks with no overlap", func() {
			d := New(throttleOp)

			for i := 0; i < 3; i++ {
				go func() {
					d.Run()
				}()
			}

			d.Wait()

			So(counter, ShouldEqual, 2)

			start1 := <-startTimes
			start2 := <-startTimes
			end1 := <-endTimes
			end2 := <-endTimes

			So(start2, ShouldHappenAfter, start1)
			So(start2, ShouldHappenAfter, end1)
			So(end2, ShouldHappenAfter, end1)

			Convey("and Wait still works after first use", func() {
				for i := 0; i < 3; i++ {
					go func() {
						d.Run()
					}()
				}

				d.Wait()

				So(counter, ShouldEqual, 4)
			})
		})
	})
}
