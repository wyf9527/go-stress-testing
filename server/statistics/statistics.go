/**
* Created by GoLand.
* User: link1st
* Date: 2019-08-15
* Time: 18:14
 */

/**
* Modified at 2020-9-28
* Author: wyf95278
 */

package statistics

import (
	"fmt"
	"go-stress-testing/model"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// 输出统计数据的时间
	exportStatisticsTime = 1 * time.Second
)

// 接收结果并处理
// 统计的时间都是纳秒，显示的时间 都是毫秒
// concurrent 并发数
func ReceivingResults(concurrent uint64, ch <-chan *model.RequestResults, wg *sync.WaitGroup) {

	defer func() {
		wg.Done()
	}()

	var (
		stopChan = make(chan bool)
	)

	// 时间
	var (
		processingTime uint64 // 处理总时间
		requestTime    uint64 // 请求总时间
		maxTime        uint64 // 最大时长
		minTime        uint64 // 最小时长
		successNum     uint64 // 成功处理数，code为0
		failureNum     uint64 // 处理失败数，code不为0
		chanIdLen      int    // 并发数
		chanIds        = make(map[uint64]bool)
	)

	statTime := uint64(time.Now().UnixNano())

	// 错误码/错误个数
	var errCode = make(map[int]int)

	// 分段计数
	var succPast, failPast uint64 = 0, 0

	//
	var latency = make([]uint64, 0)

	// 定时输出一次计算结果
	ticker := time.NewTicker(exportStatisticsTime)
	go func() {
		for {
			select {
			case <-ticker.C:
				endTime := uint64(time.Now().UnixNano())
				requestTime = endTime - statTime
				maxTimeCp, minTimeCp := atomic.SwapUint64(&maxTime, 0), atomic.SwapUint64(&minTime, 0) // 分段计算max 和 min
				succSum, failSum := atomic.LoadUint64(&successNum), atomic.LoadUint64(&failureNum)
				index := len(latency)
				go calculateData(concurrent, processingTime, requestTime, maxTimeCp, minTimeCp, succSum-succPast, failSum-failPast, succSum, failSum, latency[:index], chanIdLen, errCode)
				succPast, failPast = succSum, failSum
				latency = latency[index:] //gc
			case <-stopChan:
				// 处理完成

				return
			}
		}
	}()

	header()

	for data := range ch {
		// fmt.Println("处理一条数据", data.Id, data.Time, data.IsSucceed, data.ErrCode)
		processingTime = processingTime + data.Time

		if maxTime <= data.Time {
			maxTime = data.Time
		}

		if minTime == 0 {
			minTime = data.Time
		} else if minTime > data.Time {
			minTime = data.Time
		}

		latency = append(latency, data.Time)

		// 是否请求成功
		if data.IsSucceed == true {
			successNum = successNum + 1
		} else {
			failureNum = failureNum + 1
		}

		// 统计错误码
		if value, ok := errCode[data.ErrCode]; ok {
			errCode[data.ErrCode] = value + 1
		} else {
			errCode[data.ErrCode] = 1
		}

		if _, ok := chanIds[data.ChanId]; !ok {
			chanIds[data.ChanId] = true
			chanIdLen = len(chanIds)
		}
	}

	// 数据全部接受完成，停止定时输出统计数据
	stopChan <- true

	endTime := uint64(time.Now().UnixNano())
	requestTime = endTime - statTime

	calculateData(concurrent, processingTime, requestTime, maxTime, minTime, successNum-succPast, failureNum-failPast, successNum, failureNum, latency, chanIdLen, errCode)

	fmt.Printf("\n\n")

	fmt.Println("*************************  结果 stat  ****************************")
	fmt.Println("处理协程数量:", concurrent)
	// fmt.Println("处理协程数量:", concurrent, "程序处理总时长:", fmt.Sprintf("%.3f", float64(processingTime/concurrent)/1e9), "秒")
	fmt.Println("请求总数（并发数*请求数 -c * -n）:", successNum+failureNum, "总请求时间:", fmt.Sprintf("%.3f", float64(requestTime)/1e9),
		"秒", "successNum:", successNum, "failureNum:", failureNum)

	fmt.Println("*************************  结果 end   ****************************")

	fmt.Printf("\n\n")
}

// 计算数据
func calculateData(concurrent, processingTime, requestTime, maxTime, minTime, succ, fail, successNum, failureNum uint64, latency []uint64, chanIdLen int, errCode map[int]int) {
	if processingTime == 0 {
		processingTime = 1
	}

	var (
		qps              float64
		maxTimeFloat     float64
		minTimeFloat     float64
		requestTimeFloat float64
		PCT50            float64
		PCT95            float64
		PCT99            float64
	)

	// 平均 每个协程成功数*总协程数据/总耗时 (每秒)
	if processingTime != 0 {
		qps = float64(successNum*1e9*concurrent) / float64(processingTime)
	}

	// 纳秒=>毫秒
	maxTimeFloat = float64(maxTime) / 1e6
	minTimeFloat = float64(minTime) / 1e6
	requestTimeFloat = float64(requestTime) / 1e9

	// 计算PCT99/PCT95/PCT50
	sort.Slice(latency, func(i, j int) bool {
		if latency[i] > latency[j] {
			return false
		}
		return true
	})

	l := float64(len(latency))
	if l != 0 {
		PCT50 = float64(latency[int(l*0.5)]) / 1e6
		PCT95 = float64(latency[int(l*0.95)]) / 1e6
		PCT99 = float64(latency[int(l*0.99)]) / 1e6
	}
	// 打印的时长都为毫秒
	table(succ, fail, successNum, failureNum, errCode, qps, maxTimeFloat, minTimeFloat, requestTimeFloat, PCT50, PCT95, PCT99, chanIdLen)
}

// 打印表头信息
func header() {
	fmt.Printf("\n\n")
	// 打印的时长都为毫秒 总请数
	fmt.Println("─────┬───────┬───────┬───────┬────────┬────────┬────────┬────────┬────────┬────────┬────────┬────────┬────────")
	result := fmt.Sprintf(" 耗时│ 并发数│ 成功数│ 失败数│总成功数│总失败数│   qps  │最长耗时│最短耗时│ PCT50  │ PCT95  │ PCT99  │ 错误码")
	fmt.Println(result)
	fmt.Println("─────┼───────┼───────┼───────┼────────┼────────┼────────┼────────┼────────┼────────┼────────┼────────┼────────")

	return
}

// 打印表格
func table(succ, fail, successNum, failureNum uint64, errCode map[int]int, qps, maxTimeFloat, minTimeFloat, requestTimeFloat, pct50, pct95, pct99 float64, chanIdLen int) {
	// 打印的时长都为毫秒
	result := fmt.Sprintf("%4.0fs│%7d│%7d│%7d│%8d│%8d│%8.2f│%8.2f│%8.2f│%8.2f│%8.2f│%8.2f│%v", requestTimeFloat, chanIdLen, succ, fail, successNum, failureNum, qps, maxTimeFloat, minTimeFloat, pct50, pct95, pct99, printMap(errCode))
	fmt.Println(result)

	return
}

// 输出错误码、次数 节约字符(终端一行字符大小有限)
func printMap(errCode map[int]int) (mapStr string) {

	var (
		mapArr []string
	)
	for key, value := range errCode {
		mapArr = append(mapArr, fmt.Sprintf("%d:%d", key, value))
	}

	mapStr = strings.Join(mapArr, ";")

	return
}
