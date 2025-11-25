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
                    d='M3 3C2.44772 3 2 3.44772 2 4V20C2 20.5523 2.44772 21 3 21H21C21.5523 21 22 20.5523 22 20C22 19.4477 21.5523 19 21 19H4V4C4 3.44772 3.55228 3 3 3Z'
                    fill='currentColor'
                />
                <path
                    d='M7 16L10 13L13 16L18 11V19H6V16H7Z'
                    fill='currentColor'
                />
                <path
                    d='M7 12L10 9L13 12L18 7V15H6V12H7Z'
                    fill='currentColor'
                />
            </g>
        </svg>
    )
}

