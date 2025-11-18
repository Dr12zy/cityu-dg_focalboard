// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.
import React, {useMemo, useCallback} from 'react'
import {BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, PieChart, Pie, Cell} from 'recharts'

import {Card} from '../../blocks/card'
import {Board, IPropertyTemplate} from '../../blocks/board'
import {BoardView} from '../../blocks/boardView'
import {Utils} from '../../utils'

import './chart.scss'

type Props = {
    board: Board
    cards: Card[]
    activeView: BoardView
    readonly: boolean
    onCardClicked: (e: React.MouseEvent, card: Card) => void
    showCard: (cardId?: string) => void
    addCard: (show: boolean) => Promise<void>
}

// 颜色数组，每个任务使用不同颜色
const CHART_COLORS = [
    '#6B7280', // gray
    '#92400E', // brown
    '#F97316', // orange
    '#EAB308', // yellow
    '#22C55E', // green
    '#3B82F6', // blue
    '#A855F7', // purple
    '#EC4899', // pink
    '#EF4444', // red
    '#14B8A6', // teal
    '#F59E0B', // amber
    '#8B5CF6', // violet
]

// 将颜色名称转换为实际颜色值
const getColorValue = (colorName: string): string => {
    const colorMap: Record<string, string> = {
        'propColorGray': CHART_COLORS[0],
        'propColorBrown': CHART_COLORS[1],
        'propColorOrange': CHART_COLORS[2],
        'propColorYellow': CHART_COLORS[3],
        'propColorGreen': CHART_COLORS[4],
        'propColorBlue': CHART_COLORS[5],
        'propColorPurple': CHART_COLORS[6],
        'propColorPink': CHART_COLORS[7],
        'propColorRed': CHART_COLORS[8],
    }
    return colorMap[colorName] || CHART_COLORS[0]
}

const Chart = (props: Props): JSX.Element => {
    const {activeView, board, cards} = props

    // 准备图表数据
    const chartData = useMemo(() => {
        const filteredCards = cards.filter((c) => c.boardId === board.id)
        
        return filteredCards.map((card, index) => {
            // 为每个任务分配颜色
            const colorIndex = index % CHART_COLORS.length
            const color = CHART_COLORS[colorIndex]
            
            // 尝试从卡片的属性中获取颜色
            let cardColor = color
            for (const property of board.cardProperties) {
                if (property.type === 'select' || property.type === 'multiSelect') {
                    const propertyValue = card.fields.properties[property.id]
                    if (propertyValue) {
                        const optionId = Array.isArray(propertyValue) ? propertyValue[0] : propertyValue
                        const option = property.options.find((o) => o.id === optionId)
                        if (option && option.color) {
                            // 将颜色名称转换为CSS变量或直接使用颜色值
                            cardColor = getColorValue(option.color)
                            break
                        }
                    }
                }
            }
            
            return {
                name: card.title || `任务 ${index + 1}`,
                value: 1, // 每个任务的值，可以根据需要调整
                color: cardColor,
                cardId: card.id,
            }
        })
    }, [cards, board])

    const handleBarClick = useCallback((data: any) => {
        if (data && data.activePayload && data.activePayload[0]) {
            const payload = data.activePayload[0].payload
            if (payload && payload.cardId) {
                props.showCard(payload.cardId)
            }
        }
    }, [props.showCard])

    const handlePieClick = useCallback((data: any) => {
        if (data && data.cardId) {
            props.showCard(data.cardId)
        }
    }, [props.showCard])

    const CustomTooltip = ({active, payload}: any) => {
        if (active && payload && payload.length) {
            return (
                <div className='chart-tooltip'>
                    <p className='label'>{payload[0].payload.name}</p>
                </div>
            )
        }
        return null
    }

    if (chartData.length === 0) {
        return (
            <div className='Chart empty'>
                <p>暂无任务数据</p>
            </div>
        )
    }

    return (
        <div className='Chart'>
            <div className='chart-container'>
                <h3 className='chart-title'>任务分布图</h3>
                <ResponsiveContainer width='100%' height={400}>
                    <BarChart
                        data={chartData}
                        margin={{top: 20, right: 30, left: 20, bottom: 60}}
                    >
                        <CartesianGrid strokeDasharray='3 3' />
                        <XAxis
                            dataKey='name'
                            angle={-45}
                            textAnchor='end'
                            height={100}
                            interval={0}
                        />
                        <YAxis />
                        <Tooltip content={<CustomTooltip />} />
                        <Legend />
                        <Bar
                            dataKey='value'
                            fill='#8884d8'
                            onClick={handleBarClick}
                        >
                            {chartData.map((entry, index) => (
                                <Cell key={`cell-${index}`} fill={entry.color} />
                            ))}
                        </Bar>
                    </BarChart>
                </ResponsiveContainer>
            </div>
            
            <div className='chart-container'>
                <h3 className='chart-title'>任务饼图</h3>
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
                            onClick={handlePieClick}
                        >
                            {chartData.map((entry, index) => (
                                <Cell key={`cell-${index}`} fill={entry.color} />
                            ))}
                        </Pie>
                        <Tooltip content={<CustomTooltip />} />
                    </PieChart>
                </ResponsiveContainer>
            </div>
        </div>
    )
}

export default Chart

