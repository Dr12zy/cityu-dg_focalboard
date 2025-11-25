// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.
import React, {useMemo, useCallback} from 'react'
import {FormattedMessage} from 'react-intl'
import {
    BarChart,
    Bar,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip,
    Legend,
    ResponsiveContainer,
    PieChart,
    Pie,
    Cell,
} from 'recharts'

import {Card} from '../../blocks/card'
import {Board, IPropertyTemplate} from '../../blocks/board'
import {BoardView} from '../../blocks/boardView'

import './chart.scss'

// ============================================================================
// Types
// ============================================================================

type ChartDataPoint = {
    name: string
    value: number
    color: string
    cardId: string
}

type Props = {
    board: Board
    cards: Card[]
    activeView: BoardView
    readonly: boolean
    onCardClicked: (e: React.MouseEvent, card: Card) => void
    showCard: (cardId?: string) => void
    addCard: (show: boolean) => Promise<void>
}

type TooltipProps = {
    active?: boolean
    payload?: Array<{payload: ChartDataPoint}>
}

type BarClickData = {
    activePayload?: Array<{payload: ChartDataPoint}>
}

type PieClickData = {
    cardId?: string
}

type PropertyValue = string | string[] | number | undefined | null

// ============================================================================
// Constants
// ============================================================================

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
] as const

const COLOR_MAP: Readonly<Record<string, string>> = {
    propColorGray: CHART_COLORS[0],
    propColorBrown: CHART_COLORS[1],
    propColorOrange: CHART_COLORS[2],
    propColorYellow: CHART_COLORS[3],
    propColorGreen: CHART_COLORS[4],
    propColorBlue: CHART_COLORS[5],
    propColorPurple: CHART_COLORS[6],
    propColorPink: CHART_COLORS[7],
    propColorRed: CHART_COLORS[8],
}

const CHART_CONFIG = {
    height: 400,
    barChart: {
        margin: {top: 20, right: 30, left: 20, bottom: 60},
        xAxisHeight: 100,
        xAxisAngle: -45,
    },
    pieChart: {
        radius: 120,
        cx: '50%',
        cy: '50%',
    },
    defaultFill: '#8884d8',
    defaultValue: 1,
    minValue: 1,
} as const

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * 将颜色名称转换为实际颜色值
 */
const getColorFromName = (colorName: string): string => {
    return COLOR_MAP[colorName] ?? CHART_COLORS[0]
}

/**
 * 从卡片的选择属性中获取颜色
 */
const getColorFromSelectProperty = (
    card: Card,
    property: IPropertyTemplate,
): string | null => {
    const propertyValue = card.fields.properties[property.id]
    if (!propertyValue) {
        return null
    }

    const optionId = Array.isArray(propertyValue) ? propertyValue[0] : propertyValue
    const option = property.options?.find((o) => o.id === optionId)

    return option?.color ? getColorFromName(option.color) : null
}

/**
 * 从卡片属性中获取颜色
 * 优先使用 select/multiSelect 属性的颜色，否则使用默认颜色
 */
const getCardColor = (card: Card, board: Board, defaultColorIndex: number): string => {
    for (const property of board.cardProperties) {
        if (property.type === 'select' || property.type === 'multiSelect') {
            const color = getColorFromSelectProperty(card, property)
            if (color) {
                return color
            }
        }
    }

    return CHART_COLORS[defaultColorIndex % CHART_COLORS.length]
}

/**
 * 将属性值转换为可解析的字符串
 */
const normalizePropertyValue = (value: PropertyValue): string | null => {
    if (value === undefined || value === null || value === '') {
        return null
    }

    if (typeof value === 'number') {
        return String(value)
    }

    if (typeof value === 'string') {
        const trimmed = value.trim()
        return trimmed === '' ? null : trimmed
    }

    if (Array.isArray(value) && value.length > 0) {
        return String(value[0])
    }

    return String(value)
}

/**
 * 从卡片中提取数值
 * 使用与计算函数相同的方式来获取和解析数字属性值
 */
const getCardValue = (card: Card, numberProperty?: IPropertyTemplate): number => {
    if (!numberProperty) {
        return CHART_CONFIG.defaultValue
    }

    const propertyValue = card.fields.properties[numberProperty.id]
    const normalizedValue = normalizePropertyValue(propertyValue)

    if (!normalizedValue) {
        return CHART_CONFIG.defaultValue
    }

    const parsedValue = parseFloat(normalizedValue)

    if (isNaN(parsedValue) || parsedValue < CHART_CONFIG.minValue) {
        return CHART_CONFIG.defaultValue
    }

    return parsedValue
}

/**
 * 获取卡片的显示名称
 */
const getCardDisplayName = (card: Card, index: number): string => {
    return card.title?.trim() || `Task ${index + 1}`
}

/**
 * 转换单个卡片为图表数据点
 */
const transformCardToDataPoint = (
    card: Card,
    board: Board,
    numberProperty: IPropertyTemplate | undefined,
    index: number,
): ChartDataPoint => {
    return {
        name: getCardDisplayName(card, index),
        value: getCardValue(card, numberProperty),
        color: getCardColor(card, board, index),
        cardId: card.id,
    }
}

/**
 * 转换卡片数据为图表数据点
 */
const transformCardsToChartData = (
    cards: Card[],
    board: Board,
    chartValuePropertyId?: string,
): ChartDataPoint[] => {
    const filteredCards = cards.filter((c) => c.boardId === board.id)

    // 如果指定了 chartValuePropertyId，使用该属性；否则使用第一个数字属性
    let numberProperty: IPropertyTemplate | undefined
    if (chartValuePropertyId) {
        numberProperty = board.cardProperties.find((p) => p.id === chartValuePropertyId && p.type === 'number')
    } else {
        numberProperty = board.cardProperties.find((p) => p.type === 'number')
    }

    return filteredCards.map((card, index) =>
        transformCardToDataPoint(card, board, numberProperty, index),
    )
}

// ============================================================================
// Sub-components
// ============================================================================

const CustomTooltip = ({active, payload}: TooltipProps): JSX.Element | null => {
    if (!active || !payload?.length) {
        return null
    }

    const data = payload[0].payload

    return (
        <div className='chart-tooltip'>
            <p className='label'>
                <strong>{data.name}</strong>
            </p>
            <p className='value'>
                <FormattedMessage
                    id='Chart.value'
                    defaultMessage='Value: {value}'
                    values={{value: data.value}}
                />
            </p>
        </div>
    )
}

const EmptyState = (): JSX.Element => (
    <div className='Chart empty'>
        <p>
            <FormattedMessage
                id='Chart.noData'
                defaultMessage='No task data available'
            />
        </p>
    </div>
)

type BarChartViewProps = {
    data: ChartDataPoint[]
    onBarClick: (data: BarClickData) => void
}

type PieChartViewProps = {
    data: ChartDataPoint[]
    onPieClick: (data: PieClickData) => void
}

const BarChartView = ({data, onBarClick}: BarChartViewProps): JSX.Element => (
    <div className='chart-container'>
        <h3 className='chart-title'>
            <FormattedMessage
                id='Chart.barChartTitle'
                defaultMessage='Task Distribution Chart'
            />
        </h3>
        <ResponsiveContainer
            width='100%'
            height={CHART_CONFIG.height}
        >
            <BarChart
                data={data}
                margin={CHART_CONFIG.barChart.margin}
            >
                <CartesianGrid strokeDasharray='3 3'/>
                <XAxis
                    dataKey='name'
                    angle={CHART_CONFIG.barChart.xAxisAngle}
                    textAnchor='end'
                    height={CHART_CONFIG.barChart.xAxisHeight}
                    interval={0}
                />
                <YAxis/>
                <Tooltip content={<CustomTooltip/>}/>
                <Legend/>
                <Bar
                    dataKey='value'
                    fill={CHART_CONFIG.defaultFill}
                    onClick={onBarClick}
                >
                    {data.map((entry) => (
                        <Cell
                            key={`bar-cell-${entry.cardId}`}
                            fill={entry.color}
                        />
                    ))}
                </Bar>
            </BarChart>
        </ResponsiveContainer>
    </div>
)

const PieChartView = ({data, onPieClick}: PieChartViewProps): JSX.Element => (
    <div className='chart-container'>
        <h3 className='chart-title'>
            <FormattedMessage
                id='Chart.pieChartTitle'
                defaultMessage='Task Pie Chart'
            />
        </h3>
        <ResponsiveContainer
            width='100%'
            height={CHART_CONFIG.height}
        >
            <PieChart>
                <Pie
                    data={data}
                    cx={CHART_CONFIG.pieChart.cx}
                    cy={CHART_CONFIG.pieChart.cy}
                    labelLine={false}
                    label={({name, percent}) => `${name}: ${(percent * 100).toFixed(0)}%`}
                    outerRadius={CHART_CONFIG.pieChart.radius}
                    fill={CHART_CONFIG.defaultFill}
                    dataKey='value'
                    onClick={onPieClick}
                >
                    {data.map((entry) => (
                        <Cell
                            key={`pie-cell-${entry.cardId}`}
                            fill={entry.color}
                        />
                    ))}
                </Pie>
                <Tooltip content={<CustomTooltip/>}/>
            </PieChart>
        </ResponsiveContainer>
    </div>
)

// ============================================================================
// Main Component
// ============================================================================

const Chart = (props: Props): JSX.Element => {
    const {board, cards, activeView, showCard} = props

    // Unused props kept for interface consistency
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const {readonly, onCardClicked, addCard} = props

    // 转换卡片数据为图表数据
    const chartData = useMemo(
        () => transformCardsToChartData(cards, board, activeView.fields.chartValuePropertyId),
        [cards, board, activeView.fields.chartValuePropertyId],
    )

    // 处理柱状图点击事件
    const handleBarClick = useCallback(
        (data: BarClickData) => {
            const cardId = data?.activePayload?.[0]?.payload?.cardId
            if (cardId) {
                showCard(cardId)
            }
        },
        [showCard],
    )

    // 处理饼图点击事件
    const handlePieClick = useCallback(
        (data: PieClickData) => {
            if (data?.cardId) {
                showCard(data.cardId)
            }
        },
        [showCard],
    )

    // 如果没有数据，显示空状态
    if (chartData.length === 0) {
        return <EmptyState/>
    }

    return (
        <div className='Chart'>
            <BarChartView
                data={chartData}
                onBarClick={handleBarClick}
            />
            <PieChartView
                data={chartData}
                onPieClick={handlePieClick}
            />
        </div>
    )
}

export default React.memo(Chart)
