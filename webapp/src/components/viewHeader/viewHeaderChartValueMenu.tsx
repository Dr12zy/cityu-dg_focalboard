// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react'
import {FormattedMessage, useIntl} from 'react-intl'

import {IPropertyTemplate} from '../../blocks/board'
import {BoardView} from '../../blocks/boardView'
import mutator from '../../mutator'
import Button from '../../widgets/buttons/button'
import Menu from '../../widgets/menu'
import MenuWrapper from '../../widgets/menuWrapper'
import CheckIcon from '../../widgets/icons/check'

type Props = {
    properties: readonly IPropertyTemplate[]
    activeView: BoardView
    chartValuePropertyName?: string
}

const ViewHeaderChartValueMenu = (props: Props) => {
    const {properties, activeView, chartValuePropertyName} = props
    const intl = useIntl()

    const getNumberProperties = (): IPropertyTemplate[] => {
        return properties?.filter((o: IPropertyTemplate) => o.type === 'number')
    }

    const numberProperties = getNumberProperties()
    const defaultLabel = intl.formatMessage({
        id: 'ViewHeader.chart-value-default',
        defaultMessage: 'Default (first number property)',
    })

    const currentPropertyName = chartValuePropertyName || defaultLabel

    return (
        <MenuWrapper>
            <Button>
                <FormattedMessage
                    id='ViewHeader.chart-value-by'
                    defaultMessage='Chart value: {property}'
                    values={{
                        property: (
                            <span
                                style={{color: 'rgb(var(--center-channel-color-rgb))'}}
                                id='chartValueLabel'
                            >
                                {currentPropertyName}
                            </span>
                        ),
                    }}
                />
            </Button>
            <Menu>
                <Menu.Text
                    key='default'
                    id=''
                    name={defaultLabel}
                    rightIcon={!activeView.fields.chartValuePropertyId ? <CheckIcon/> : undefined}
                    onClick={() => {
                        if (!activeView.fields.chartValuePropertyId) {
                            return
                        }
                        mutator.changeViewChartValuePropertyId(
                            activeView.boardId,
                            activeView.id,
                            activeView.fields.chartValuePropertyId,
                            undefined,
                        )
                    }}
                />
                {numberProperties.length > 0 && numberProperties.map((numberProp: IPropertyTemplate) => (
                    <Menu.Text
                        key={numberProp.id}
                        id={numberProp.id}
                        name={numberProp.name}
                        rightIcon={activeView.fields.chartValuePropertyId === numberProp.id ? <CheckIcon/> : undefined}
                        onClick={(id) => {
                            if (activeView.fields.chartValuePropertyId === id) {
                                return
                            }
                            mutator.changeViewChartValuePropertyId(
                                activeView.boardId,
                                activeView.id,
                                activeView.fields.chartValuePropertyId,
                                id,
                            )
                        }}
                    />
                ))}
            </Menu>
        </MenuWrapper>
    )
}

export default React.memo(ViewHeaderChartValueMenu)

