// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.
/* eslint-disable */
import React, {useState, useRef, useEffect} from 'react'
import {FormattedMessage} from 'react-intl'

import {useAppSelector} from '../../store/hooks'
import {getMe} from '../../store/users'
import octoClient, {AIChatMessage} from '../../octoClient'

import Modal from '../modal'
import CloseIcon from '../../widgets/icons/close'

import './taskAIChat.scss'

type Props = {
    onClose: () => void
}

type ChatMessage = {
    id: string
    text: string
    isUser: boolean
    pending?: boolean
}

const createMessageId = () => `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

const buildAIHistory = (history: ChatMessage[]): AIChatMessage[] => history
    .filter((msg) => !msg.pending)
    .map((msg) => ({
        role: msg.isUser ? 'user' : 'assistant',
        content: msg.text,
    }))

function TaskAIChat(props: Props) {
    const [messages, setMessages] = useState<ChatMessage[]>([])
    const [input, setInput] = useState('')
    const [showWelcomeMessage, setShowWelcomeMessage] = useState(true)
    const [selectedFile, setSelectedFile] = useState<File | null>(null)
    const [isDragOver, setIsDragOver] = useState(false)
    const [isSending, setIsSending] = useState(false)
    const [error, setError] = useState<string | null>(null)

    const textareaRef = useRef<HTMLTextAreaElement>(null)

    const me = useAppSelector(getMe)

    // Auto-resize textarea
    useEffect(() => {
        if (textareaRef.current) {
            textareaRef.current.style.height = 'auto'
            textareaRef.current.style.height = textareaRef.current.scrollHeight + 'px'
        }
    }, [input])

    const allowedFileTypes = [
        'image/jpeg', 'image/png', 'image/gif', 'image/webp',
        'application/pdf',
        'application/msword',
        'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        'text/plain'
    ]

    const validateFile = (file: File): boolean => {
        if (!allowedFileTypes.includes(file.type)) {
            alert('不支持的文件类型。请上传图片、PDF、Word文档或文本文件。')
            return false
        }
        if (file.size > 10 * 1024 * 1024) { // 10MB limit
            alert('文件大小不能超过10MB。')
            return false
        }
        return true
    }

    const handleSend = async () => {
        if (isSending) {
            return
        }

        if (!input.trim() && !selectedFile) {
            return
        }

        const messageText = input.trim() || (selectedFile ? `Uploaded: ${selectedFile.name}` : '')
        const userMessage: ChatMessage = {id: createMessageId(), text: messageText, isUser: true}
        const placeholderId = createMessageId()
        const pendingMessage: ChatMessage = {id: placeholderId, text: 'AI is thinking   …', isUser: false, pending: true}

        setMessages((prev) => [...prev, userMessage, pendingMessage])
        setInput('')
        setSelectedFile(null)
        setShowWelcomeMessage(false)
        setError(null)
        setIsSending(true)

        const historyPayload = buildAIHistory([...messages, userMessage])

        try {
            const response = await octoClient.sendAIChat({
                message: messageText,
                messages: historyPayload,
            })

            setMessages((prev) => prev.map((msg) => {
                if (msg.id === placeholderId) {
                    return {
                        ...msg,
                        text: response.message || 'AI 暂无回复',
                        pending: false,
                    }
                }
                return msg
            }))
        } catch (err) {
            const errorText = err instanceof Error ? err.message : 'AI 服务暂时不可用'
            setMessages((prev) => prev.map((msg) => {
                if (msg.id === placeholderId) {
                    return {
                        ...msg,
                        text: `AI 出错：${errorText}`,
                        pending: false,
                    }
                }
                return msg
            }))
            setError(errorText)
        } finally {
            setIsSending(false)
        }
    }

    const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0]
        if (file && validateFile(file)) {
            setSelectedFile(file)
        }
    }

    const handleFileButtonClick = () => {
        document.getElementById('file-input')?.click()
    }

    const handleDragOver = (e: React.DragEvent) => {
        e.preventDefault()
        e.stopPropagation()
        setIsDragOver(true)
    }

    const handleDragLeave = (e: React.DragEvent) => {
        e.preventDefault()
        e.stopPropagation()

        // 检查是否真的离开了聊天窗口区域
        const rect = e.currentTarget.getBoundingClientRect()
        const x = e.clientX
        const y = e.clientY

        if (x < rect.left || x > rect.right || y < rect.top || y > rect.bottom) {
            setIsDragOver(false)
        }
    }

    const handleDrop = (e: React.DragEvent) => {
        e.preventDefault()
        e.stopPropagation()
        setIsDragOver(false)

        const files = Array.from(e.dataTransfer.files)
        if (files.length > 0) {
            const file = files[0]
            if (validateFile(file)) {
                setSelectedFile(file)
            }
        }
    }

    const handleKeyPress = (e: React.KeyboardEvent) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault()
            void handleSend()
        }
        // Shift + Enter will create a new line (default behavior)
    }

    return (
        <Modal
            onClose={props.onClose}
            closeOnBlur={false}
        >
            <div
                className={`task-ai-chat ${isDragOver ? 'drag-over' : ''}`}
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onDrop={handleDrop}
            >
                <div className='task-ai-chat-header'>
                    <h3>
                        <FormattedMessage
                            id='TaskAIChat.title'
                            defaultMessage='Task AI'
                        />
                    </h3>
                    <button
                        className='close-button'
                        onClick={props.onClose}
                    >
                        <CloseIcon />
                    </button>
                </div>
                <div className='task-ai-chat-messages'>
                    {messages.length === 0 && showWelcomeMessage && me ? (
                        <div className='welcome-message'>
                            Hi, {me.username}!
                        </div>
                    ) : (
                        messages.map((msg) => (
                            <div
                                key={msg.id}
                                className={`message ${msg.isUser ? 'user' : 'ai'} ${msg.pending ? 'pending' : ''}`}
                            >
                                {msg.pending ? 'AI is thinking…' : msg.text}
                            </div>
                        ))
                    )}
                </div>
                <div className='task-ai-chat-input-area'>
                    <div className='task-ai-chat-input'>
                        <textarea
                            ref={textareaRef}
                            value={input}
                            onChange={(e) => setInput(e.target.value)}
                            onKeyDown={handleKeyPress}
                            placeholder='Ask anything about your task...'
                            rows={1}
                            style={{ resize: 'none', minHeight: '40px', maxHeight: '160px' }}
                        />
                        <input
                            id='file-input'
                            type='file'
                            accept='image/*,.pdf,.doc,.docx,.txt'
                            onChange={handleFileSelect}
                            style={{display: 'none'}}
                        />
                        <button onClick={handleFileButtonClick} className='file-upload-button'>
                            📎
                        </button>
                        <button
                            onClick={handleSend}
                            disabled={isSending || (!input.trim() && !selectedFile)}
                        >
                            <FormattedMessage
                                id='TaskAIChat.send'
                                defaultMessage='发送'
                            />
                        </button>
                    </div>
                    {error && (
                        <div className='task-ai-chat-error'>
                            {error}
                        </div>
                    )}
                    {selectedFile && (
                        <div className='selected-file'>
                            <span>Selected: {selectedFile.name}</span>
                            <button onClick={() => setSelectedFile(null)}>×</button>
                        </div>
                    )}
                </div>
            </div>
        </Modal>
    )
}

export default TaskAIChat