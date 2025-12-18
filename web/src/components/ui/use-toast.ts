import { useState, useEffect } from "react"

const TOAST_LIMIT = 1

type ToastType = {
    id: string
    title?: string
    description?: string
    action?: React.ReactNode
    variant?: "default" | "destructive"
}

const listeners: Array<(state: ToastType[]) => void> = []
let memoryState: ToastType[] = []

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function dispatch(action: any) {
    memoryState = reducer(memoryState, action)
    listeners.forEach((listener) => {
        listener(memoryState)
    })
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function reducer(state: ToastType[], action: any) {
    switch (action.type) {
        case "ADD_TOAST":
            return [action.toast, ...state].slice(0, TOAST_LIMIT)
        case "DISMISS_TOAST":
            return state.filter((t) => t.id !== action.toastId)
        case "REMOVE_TOAST":
            if (action.toastId === undefined) {
                return []
            }
            return state.filter((t) => t.id !== action.toastId)
    }
    return state
}

export function toast({ ...props }: Omit<ToastType, "id">) {
    const id = genId()

    const update = (props: ToastType) =>
        dispatch({
            type: "UPDATE_TOAST",
            toast: { ...props, id },
        })
    const dismiss = () => dispatch({ type: "DISMISS_TOAST", toastId: id })

    dispatch({
        type: "ADD_TOAST",
        toast: {
            ...props,
            id,
            open: true,
            onOpenChange: (open: boolean) => {
                if (!open) dismiss()
            },
        },
    })

    return {
        id: id,
        dismiss,
        update,
    }
}

export function useToast() {
    const [state, setState] = useState<ToastType[]>(memoryState)

    useEffect(() => {
        listeners.push(setState)
        return () => {
            const index = listeners.indexOf(setState)
            if (index > -1) {
                listeners.splice(index, 1)
            }
        }
    }, [state])

    return {
        toast,
        dismiss: (toastId?: string) => dispatch({ type: "DISMISS_TOAST", toastId }),
        toasts: state,
    }
}

let count = 0
function genId() {
    count = (count + 1) % Number.MAX_SAFE_INTEGER
    return count.toString()
}
