// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react'

import './chart.scss'

export default function ChartIcon(): JSX.Element {
    return (
        <svg
            width='24'
            height='24'
            viewBox='0 0 24 24'
            fill='currentColor'
            xmlns='http://www.w3.org/2000/svg'
            className='ChartIcon Icon'
        >
            <g opacity='0.8'>
                <path
                    fillRule='evenodd'
                    clipRule='evenodd'
                    d='M3 3C3 2.44772 3.44772 2 4 2H20C20.5523 2 21 2.44772 21 3V21C21 21.5523 20.5523 22 20 22H4C3.44772 22 3 21.5523 3 21V3ZM5 4V20H19V4H5ZM7 16L7 12H9L9 16H7ZM11 16L11 8H13L13 16H11ZM15 16L15 14H17L17 16H15Z'
                    fill='currentColor'
                />
            </g>
        </svg>
    )
}

