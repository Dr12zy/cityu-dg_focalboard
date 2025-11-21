// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useState} from 'react'
import {FormattedMessage} from 'react-intl'
import {
    BarChart,
    Bar,
    LineChart,
    Line,
    PieChart,
    Pie,
    Cell,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip,
    Legend,
    ResponsiveContainer,
} from 'recharts'

import {Card} from '../../blocks/card'
import {Board, IPropertyTemplate} from '../../blocks/board'
import {BoardView} from '../../blocks/boardView'
import {getVisibleAndHiddenGroups} from '../../boardUtils'
import {Constants} from '../../constants'

import './chartView.scss'

type Props = {
    board: Board
    cards: Card[]
    activeView: BoardView
    readonly: boolean
    groupByProperty?: IPropertyTemplate
    visibleGroups: Array<{option: {id: string, value: string, color: string}, cards: Card[]}>
}

type ChartType = 'bar' | 'line' | 'pie'

const COLORS = ['#0088FE', '#00C49F', '#FFBB28', '#FF8042', '#8884d8', '#82ca9d', '#ffc658', '#ff7c7c']

const ChartView = (props: Props): JSX.Element => {
    const {activeView, board, cards, visibleGroups} = props
    const [chartType, setChartType] = useState<ChartType>('bar')

    // 准备图表数据
    const chartData = useMemo(() => {
        // 如果有分组属性，使用分组数据
        if (props.groupByProperty && visibleGroups && visibleGroups.length > 0) {
            return visibleGroups.map((group) => ({
                name: group.option.value || '未分组',
                value: group.cards.length,
                color: group.option.color || COLORS[0],
            }))
        }

        // 如果没有分组，尝试按第一个选择属性分组
        const selectProperty = board.cardProperties.find((p) => p.type === 'select' || p.type === 'multiSelect')
        if (selectProperty) {
            const groupedData: Record<string, number> = {}
            cards.forEach((card) => {
                const value = card.fields.properties[selectProperty.id]
                if (value) {
                    const key = Array.isArray(value) ? value.join(', ') : value
                    groupedData[key] = (groupedData[key] || 0) + 1
                } else {
                    groupedData['未设置'] = (groupedData['未设置'] || 0) + 1
                }
            })

            return Object.entries(groupedData).map(([name, value], index) => ({
                name,
                value,
                color: COLORS[index % COLORS.length],
            }))
        }

        // 如果都没有，按总数统计
        return [{
            name: '全部任务',
            value: cards.length,
            color: COLORS[0],
        }]
    }, [visibleGroups, props.groupByProperty, board.cardProperties, cards])

    const renderChart = () => {
        if (chartData.length === 0) {
            return (
                <div className='chart-empty'>
                    <FormattedMessage
                        id='ChartView.noData'
                        defaultMessage='暂无数据可显示'
                    />
                </div>
            )
        }

        switch (chartType) {
        case 'bar':
            return (
                <ResponsiveContainer width='100%' height={400}>
                    <BarChart data={chartData}>
                        <CartesianGrid strokeDasharray='3 3'/>
                        <XAxis dataKey='name'/>
                        <YAxis/>
                        <Tooltip/>
                        <Legend/>
                        <Bar dataKey='value' fill='#8884d8'/>
                    </BarChart>
                </ResponsiveContainer>
            )
        case 'line':
            return (
                <ResponsiveContainer width='100%' height={400}>
                    <LineChart data={chartData}>
                        <CartesianGrid strokeDasharray='3 3'/>
                        <XAxis dataKey='name'/>
                        <YAxis/>
                        <Tooltip/>
                        <Legend/>
                        <Line type='monotone' dataKey='value' stroke='#8884d8'/>
                    </LineChart>
                </ResponsiveContainer>
            )
        case 'pie':
            return (
                <ResponsiveContainer width='100%' height={400}>
                    <PieChart>
                        <Pie
                            data={chartData}
                            cx='50%'
                            cy='50%'
                            labelLine={false}
                            label={({name, percent}) => `${name}: ${(percent * 100).toFixed(0)}%`}
                            outerRadius={120}
                            fill='#8884d8'
                            dataKey='value'
                        >
                            {chartData.map((entry, index) => (
                                <Cell key={`cell-${index}`} fill={entry.color || COLORS[index % COLORS.length]}/>
                            ))}
                        </Pie>
                        <Tooltip/>
                        <Legend/>
                    </PieChart>
                </ResponsiveContainer>
            )
        default:
            return null
        }
    }

    return (
        <div className='ChartView'>
            <div className='chart-header'>
                <div className='chart-title'>
                    <FormattedMessage
                        id='ChartView.title'
                        defaultMessage='任务统计图表'
                    />
                </div>
                <div className='chart-type-selector'>
                    <button
                        className={chartType === 'bar' ? 'active' : ''}
                        onClick={() => setChartType('bar')}
                    >
                        <FormattedMessage id='ChartView.bar' defaultMessage='柱状图'/>
                    </button>
                    <button
                        className={chartType === 'line' ? 'active' : ''}
                        onClick={() => setChartType('line')}
                    >
                        <FormattedMessage id='ChartView.line' defaultMessage='折线图'/>
                    </button>
                    <button
                        className={chartType === 'pie' ? 'active' : ''}
                        onClick={() => setChartType('pie')}
                    >
                        <FormattedMessage id='ChartView.pie' defaultMessage='饼图'/>
                    </button>
                </div>
            </div>
            <div className='chart-container'>
                {renderChart()}
            </div>
        </div>
    )
}

export default ChartView

