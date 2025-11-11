// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.
import React, {useRef, useEffect, useCallback} from 'react'

import IconButton from '../widgets/buttons/iconButton'
import CloseIcon from '../widgets/icons/close'
import './modal.scss'

type Props = {
    onClose: () => void
    position?: 'top'|'bottom'|'bottom-right'
    closeOnBlur?: boolean
    children: React.ReactNode
}

const Modal = (props: Props): JSX.Element => {
    const node = useRef<HTMLDivElement>(null)

    const {position, onClose, closeOnBlur = true, children} = props

    const closeOnBlurHandler = useCallback((e: Event) => {
        if (e.target && node.current?.contains(e.target as Node)) {
            return
        }
        onClose()
    }, [onClose])

    useEffect(() => {
        if (closeOnBlur) {
            document.addEventListener('click', closeOnBlurHandler, true)
            return () => {
                document.removeEventListener('click', closeOnBlurHandler, true)
            }
        }
    }, [closeOnBlur, closeOnBlurHandler])

    return (
        <div
            className={'Modal ' + (position || 'bottom')}
            ref={node}
        >
            <div className='toolbar hideOnWidescreen'>
                <IconButton
                    onClick={() => onClose()}
                    icon={<CloseIcon/>}
                    title={'Close'}
                />
            </div>
            {children}
        </div>
    )
}

export default React.memo(Modal)
