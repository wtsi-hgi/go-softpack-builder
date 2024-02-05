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

package reindex

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestThrottle(t *testing.T) {
	Convey("Given a Throttle op", t, func() {
		calls := make(chan time.Time, 50)
		callLength := 0 * time.Millisecond
		throttleOp := func() {
			<-time.After(callLength)
			calls <- time.Now()
		}

		period := 10 * time.Millisecond
		tolerance := 2 * time.Millisecond

		Convey("Throttle lets you do things only once every period of time", func() {
			started := time.Now()

			throt := NewThrottle(throttleOp, period, false)
			throt.Start()
			defer throt.Stop()

			called := <-calls
			So(called, ShouldHappenWithin, tolerance, started)

			called2 := <-calls
			So(called2, ShouldHappenBetween, called.Add(period), called.Add(period*2))

			Convey("Stop() stops doing the thing", func() {
				throt.Stop()

				select {
				case <-calls:
					So(false, ShouldBeTrue)
				case <-time.After(period * 2):
					So(true, ShouldBeTrue)
				}
			})
		})

		Convey("Throttle executes long-running tasks with no overlap, after the next full period", func() {
			callLength = 15 * time.Millisecond
			So(callLength, ShouldBeGreaterThan, period)
			started := time.Now()

			throt := NewThrottle(throttleOp, period, false)
			throt.Start()
			defer throt.Stop()

			called := <-calls
			So(called, ShouldHappenWithin, tolerance, started.Add(callLength))

			called2 := <-calls
			So(called2, ShouldHappenWithin, tolerance, started.Add(period*2).Add(callLength))

			called3 := <-calls
			So(called3, ShouldHappenWithin, tolerance, started.Add(period*4).Add(callLength))
		})

		Convey("Throttle only runs the task if it has received a signal since it last ran", func() {
			started := time.Now()

			throt := NewThrottle(throttleOp, period, true)
			throt.Start()
			defer throt.Stop()

			<-time.After(20 * period)
			select {
			case <-calls:
				So(true, ShouldBeFalse)
			default:
				So(true, ShouldBeTrue)
			}

			throt.Signal()
			called := <-calls
			So(called, ShouldHappenWithin, tolerance, started.Add(21*period))
		})
	})
}
