import { Children, cloneElement, CSSProperties, FormEvent, isValidElement, KeyboardEvent, ReactElement, ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import rehypeRaw from 'rehype-raw';
import remarkGfm from 'remark-gfm';
import './App.css';
import appIcon from './assets/images/appicon.png';
import { createTranslator, languageOptions, normalizeLanguage, Translator, Language, TranslationKey } from './i18n';
import {
    ActivateProcess,
    AssignSnippetLabel,
    BrowseAIPromptApp,
    CancelAIRequest,
    ConfirmLabelDeletion,
    ConfirmSnippetDeletion,
    CreateAIPromptProfile,
    CreateLabel,
    CreateSnippet,
    DeleteAIPromptProfile,
    DeleteLabel,
    DeleteSnippet,
    GetAISettings,
    GetAIPromptSettings,
    GetDashboard,
    GetGeneralSettings,
    GetPlatformStatus,
    GetTTSModelStatus,
    StartTTSModelDownload,
    CancelTTSModelDownload,
    ListLabels,
    ListSnippetsByLabel,
    RequestAccessibilityPermission,
    ReplaceSelectedText,
    RunAIAssist,
    SaveAISettings,
    SaveCommonAIPromptRule,
    SaveGeneralSettings,
    Speak,
    TestSpeak,
    StopSpeaking,
    SetLabelSnippetsEnabled,
    ToggleSnippet,
    UpdateAIPromptProfile,
    UpdateLabel,
    UpdateSnippet,
} from "../bindings/dkst-text-flow/app";
import { Application, Events, Window } from "@wailsio/runtime";

import { Snippet, SnippetInput, Label, LabelInput, DashboardStats, DailyTypingStat } from "../bindings/dkst-text-flow/internal/storage/models";
import { Status as PlatformStatus } from "../bindings/dkst-text-flow/internal/platform/models";
import { Settings as AISettings } from "../bindings/dkst-text-flow/internal/ai/models";
import { GeneralSettings, AIPromptRule, AIPromptProfile, AIPromptProfileInput, AIPromptSettings } from "../bindings/dkst-text-flow/models";

type AIInvocationContext = {
    kind: string;
    text: string;
    filePath: string;
    label: string;
    sourceProcessId: number;
    appName: string;
    appBundleId: string;
};

const emptyInput: SnippetInput = {
    labelId: 0,
    shortcut: '',
    title: '',
    content: '',
    contentType: 'plain',
    enabled: true,
    caseSensitive: false,
    usePaste: false,
    expandMode: 'delimiter',
};

const emptyLabelInput: LabelInput = {
    name: '',
    description: '',
    color: '#153e75',
};

const contentTypeOptions: { value: string; labelKey: TranslationKey }[] = [
    { value: 'plain', labelKey: 'plain' },
    { value: 'rich', labelKey: 'rich' },
];

function normalizeContentType(contentType: string) {
    return contentTypeOptions.some((option) => option.value === contentType) ? contentType : 'plain';
}

function contentTypeLabel(contentType: string, t: Translator) {
    const normalized = normalizeContentType(contentType);
    const labelKey = contentTypeOptions.find((option) => option.value === normalized)?.labelKey ?? 'plain';
    return t(labelKey);
}

function formatCount(value: number) {
    return new Intl.NumberFormat().format(value);
}

function hasUnsupportedShortcutCharacters(shortcut: string) {
    return /[^\x21-\x7E]/.test(shortcut.trim());
}

function shouldSuggestPaste(content: string) {
    return content.includes('\n') || content.length >= 240;
}

function isDuplicateShortcutError(err: unknown) {
    return String(err).toLowerCase().includes('unique constraint failed: snippets.shortcut');
}

const snippetTokens: { labelKey: TranslationKey; value: string }[] = [
    { labelKey: 'tokenPaste', value: '{{clipboard}}' },
    { labelKey: 'tokenDate', value: '{{date:2006-01-02}}' },
    { labelKey: 'tokenTime', value: '{{time:15:04}}' },
    { labelKey: 'tab', value: '{{tab}}' },
    { labelKey: 'return', value: '{{return}}' },
    { labelKey: 'esc', value: '{{esc}}' },
    { labelKey: 'tokenSpaceBar', value: '{{space}}' },
    { labelKey: 'home', value: '{{home}}' },
    { labelKey: 'end', value: '{{end}}' },
    { labelKey: 'pageUp', value: '{{pageup}}' },
    { labelKey: 'pageDown', value: '{{pagedown}}' },
    { labelKey: 'up', value: '{{up}}' },
    { labelKey: 'down', value: '{{down}}' },
    { labelKey: 'left', value: '{{left}}' },
    { labelKey: 'right', value: '{{right}}' },
];

const noSoundName = 'None';
const soundAssetModules = import.meta.glob('./assets/sounds/*', {
    eager: true,
    as: 'url',
}) as Record<string, string>;
const soundOptions = [
    noSoundName,
    ...Object.keys(soundAssetModules)
        .map((path) => path.split('/').pop() ?? '')
        .filter(Boolean)
        .map((filename) => filename.replace(/\.[^.]+$/, ''))
        .sort((left, right) => left.localeCompare(right, undefined, { numeric: true })),
];
const soundURLs = Object.fromEntries(
    Object.entries(soundAssetModules).map(([path, url]) => {
        const filename = path.split('/').pop() ?? '';
        return [filename.replace(/\.[^.]+$/, ''), url];
    }),
) as Record<string, string>;

const emptyPromptRule: AIPromptRule = {
    useSelectedText: true,
    runWithoutSelection: true,
    selectedTextPrompt: '',
    noSelectionPrompt: '',
};

const hotkeyCodeLabels: Record<string, string> = {
    Space: 'Space',
    Tab: 'Tab',
    Enter: 'Enter',
    Escape: 'Esc',
    ArrowUp: 'Up',
    ArrowDown: 'Down',
    ArrowLeft: 'Left',
    ArrowRight: 'Right',
    Backquote: '`',
    Minus: '-',
    Equal: '=',
    BracketLeft: '[',
    BracketRight: ']',
    Backslash: '\\',
    Semicolon: ';',
    Quote: "'",
    Comma: ',',
    Period: '.',
    Slash: '/',
};

function formatCapturedHotkey(event: KeyboardEvent): string {
    if (event.key === 'Escape') {
        return '';
    }

    const key = keyLabelFromKeyboardEvent(event);
    if (['Control', 'Shift', 'Alt', 'Meta', 'Command'].includes(key)) {
        return '';
    }

    const parts = [];
    if (event.metaKey) parts.push('Cmd');
    if (event.ctrlKey) parts.push('Ctrl');
    if (event.altKey) parts.push('Option');
    if (event.shiftKey) parts.push('Shift');
    parts.push(key);

    return parts.length >= 2 ? parts.join('+') : '';
}

function keyLabelFromKeyboardEvent(event: KeyboardEvent): string {
    if (event.code.startsWith('Key')) {
        return event.code.slice(3).toUpperCase();
    }
    if (event.code.startsWith('Digit')) {
        return event.code.slice(5);
    }
    if (event.code.startsWith('Numpad')) {
        return event.code.slice(6);
    }
    return hotkeyCodeLabels[event.code] ?? (event.key.length === 1 ? event.key.toUpperCase() : event.key);
}

function aiStatusLabel(elapsedMs: number, t: Translator) {
    if (elapsedMs < 1000) {
        return t('preparingRequest');
    }
    if (elapsedMs < 4000) {
        return t('waitingForModel');
    }
    return t('generatingResponse');
}

type MarkdownAlertType = 'tip' | 'important' | 'caution';

const markdownAlertLabels: Record<MarkdownAlertType, string> = {
    tip: 'Tip',
    important: 'Important',
    caution: 'Caution',
};

function renderMarkdownBlockquote(children: ReactNode) {
    const childArray = Children.toArray(children);
    const firstParagraphIndex = childArray.findIndex((child) => (
        isValidElement<{ children?: ReactNode }>(child) && child.type === 'p'
    ));
    const firstChild = firstParagraphIndex >= 0 ? childArray[firstParagraphIndex] : undefined;

    if (!isValidElement<{ children?: ReactNode }>(firstChild) || firstChild.type !== 'p') {
        return <blockquote>{children}</blockquote>;
    }

    const paragraphChildren = Children.toArray(firstChild.props.children);
    const firstParagraphNode = paragraphChildren[0];

    if (typeof firstParagraphNode !== 'string') {
        return <blockquote>{children}</blockquote>;
    }

    const markerMatch = firstParagraphNode.match(/^\s*\[!(TIP|IMPORTANT|CAUTION)\]\s*/i);

    if (!markerMatch) {
        return <blockquote>{children}</blockquote>;
    }

    const alertType = markerMatch[1].toLowerCase() as MarkdownAlertType;
    const remainingText = firstParagraphNode.slice(markerMatch[0].length);
    const nextParagraphChildren = remainingText
        ? [remainingText, ...paragraphChildren.slice(1)]
        : paragraphChildren.slice(1);
    const bodyChildren = nextParagraphChildren.length
        ? [
            cloneElement(
                firstChild as ReactElement<{ children?: ReactNode }>,
                undefined,
                ...nextParagraphChildren,
            ),
            ...childArray.slice(firstParagraphIndex + 1),
        ]
        : childArray.slice(firstParagraphIndex + 1);

    return (
        <div className={`markdown-alert markdown-alert-${alertType}`}>
            <p className="markdown-alert-title">{markdownAlertLabels[alertType]}</p>
            {childArray.slice(0, firstParagraphIndex)}
            {bodyChildren}
        </div>
    );
}

const aboutMarkdownComponents = {
    blockquote({ children }: { children?: ReactNode }) {
        return renderMarkdownBlockquote(children);
    },
};

function App() {
    const isHUD = useMemo(() => new URLSearchParams(window.location.search).get('mode') === 'hud', []);
    const [activeView, setActiveView] = useState<'snippets' | 'dashboard' | 'aiPrompts' | 'ai' | 'settings' | 'about'>(isHUD ? 'ai' : 'snippets');
    const [windowMode, setWindowMode] = useState<'main' | 'hud'>(isHUD ? 'hud' : 'main');
    const [snippets, setSnippets] = useState<Snippet[]>([]);
    const [labels, setLabels] = useState<Label[]>([]);
    const [stats, setStats] = useState<DashboardStats>({
        totalExpansions: 0,
        todayExpansions: 0,
        snippetCount: 0,
        enabledCount: 0,
        todayTypingCount: 0,
        averageDailyTyping: 0,
        typingHistory: [],
        topSnippets: [],
    });
    const [platformStatus, setPlatformStatus] = useState<PlatformStatus | null>(null);
    const [generalSettings, setGeneralSettings] = useState<GeneralSettings | null>(null);
    const [aiSettings, setAISettings] = useState<AISettings | null>(null);
    const [modelStatus, setModelStatus] = useState<any>({
        isDownloaded: false,
        status: 'idle',
        progress: 0,
        currentFile: '',
    });
    const [aiPromptSettings, setAIPromptSettings] = useState<AIPromptSettings | null>(null);
    const [selectedPromptID, setSelectedPromptID] = useState('common');
    const [promptSaving, setPromptSaving] = useState(false);
    const [generalSaving, setGeneralSaving] = useState(false);
    const [aiSaving, setAISaving] = useState(false);
    const [query, setQuery] = useState('');
    const [selectedLabelID, setSelectedLabelID] = useState(0);
    const [selectedID, setSelectedID] = useState<number | null>(null);
    const [detailMode, setDetailMode] = useState<'all' | 'label' | 'snippet'>('all');
    const [isModalOpen, setIsModalOpen] = useState(false);
    const [editingSnippet, setEditingSnippet] = useState<Snippet | null>(null);
    const [form, setForm] = useState<SnippetInput>(emptyInput);
    const [labelForm, setLabelForm] = useState<LabelInput>(emptyLabelInput);
    const [error, setError] = useState('');
    const [shortcutWarning, setShortcutWarning] = useState('');
    const [pastePreferenceTouched, setPastePreferenceTouched] = useState(false);
    const [pasteWarning, setPasteWarning] = useState('');
    const [aboutMarkdown, setAboutMarkdown] = useState('');
    const [aboutLoading, setAboutLoading] = useState(true);
    const [aboutError, setAboutError] = useState('');
    const [aiPrompt, setAIPrompt] = useState('');
    const [aiResult, setAIResult] = useState('');
    const [aiReplacement, setAIReplacement] = useState('');
    const [aiRunning, setAIRunning] = useState(false);
    const [aiElapsedMs, setAIElapsedMs] = useState(0);
    const [recordingHotkey, setRecordingHotkey] = useState(false);
    const [recordingTtsHotkey, setRecordingTtsHotkey] = useState(false);
    const [hasBeenInvoked, setHasBeenInvoked] = useState(false);
    const [aiContext, setAIContext] = useState<AIInvocationContext>({
        kind: 'none',
        text: '',
        filePath: '',
        label: 'No Context',
        sourceProcessId: 0,
        appName: '',
        appBundleId: '',
    });
    const aiPromptRef = useRef<HTMLTextAreaElement | null>(null);
    const snippetContentRef = useRef<HTMLTextAreaElement | null>(null);
    const shortcutInputRef = useRef<HTMLInputElement | null>(null);
    const soundNameRef = useRef(noSoundName);
    const soundAudioRef = useRef<HTMLAudioElement | null>(null);

    const selectedSnippet = useMemo(() => {
        return snippets.find((snippet) => snippet.id === selectedID) ?? null;
    }, [selectedID, snippets]);

    const selectedLabel = useMemo(() => {
        return labels.find((label) => label.id === selectedLabelID) ?? null;
    }, [labels, selectedLabelID]);

    const language = (generalSettings?.language ?? 'en') as Language;
    const t = useMemo(() => createTranslator(language), [language]);

    useEffect(() => {
        const soundName = generalSettings?.soundName || noSoundName;
        soundNameRef.current = soundName;
        if (soundName === noSoundName || !soundURLs[soundName]) {
            soundAudioRef.current = null;
            return;
        }
        const audio = new Audio(soundURLs[soundName]);
        audio.preload = 'auto';
        audio.load();
        soundAudioRef.current = audio;
    }, [generalSettings?.soundName]);

    const allLabel = useMemo<Label>(() => ({
        id: 0,
        name: t('all'),
        description: t('snippetsEnabledCount', { snippets: stats.snippetCount, enabled: stats.enabledCount }),
        color: '#667386',
        snippetCount: stats.snippetCount,
        enabledCount: stats.enabledCount,
        createdAt: '',
        updatedAt: '',
    }), [stats.enabledCount, stats.snippetCount, t]);

    async function refresh(search = query, labelID = selectedLabelID) {
        const [nextLabels, nextSnippets, nextStats, nextStatus] = await Promise.all([
            ListLabels(),
            ListSnippetsByLabel(search, labelID),
            GetDashboard(),
            GetPlatformStatus(),
        ]);
        setLabels(nextLabels || []);
        const snippetsList = nextSnippets || [];
        setSnippets(snippetsList);
        setStats({
            ...nextStats,
            typingHistory: nextStats.typingHistory || [],
            topSnippets: nextStats.topSnippets || [],
        });
        setPlatformStatus(nextStatus);
        if (snippetsList.length > 0 && !snippetsList.some((snippet) => snippet.id === selectedID)) {
            setSelectedID(snippetsList[0].id);
            if (detailMode === 'snippet') {
                setDetailMode('snippet');
            }
        }
        if (snippetsList.length === 0 && detailMode === 'snippet') {
            setSelectedID(null);
            setDetailMode(labelID > 0 ? 'label' : 'all');
        }
    }

    useEffect(() => {
        refresh('').catch((err) => setError(String(err)));
        GetGeneralSettings().then((settings) => setGeneralSettings(normalizeGeneralSettings(settings))).catch((err) => setError(String(err)));
        GetAISettings().then((settings) => setAISettings(normalizeAISettings(settings))).catch((err) => setError(String(err)));
        GetAIPromptSettings().then((settings) => setAIPromptSettings(normalizeAIPromptSettings(settings))).catch((err) => setError(String(err)));
    }, []);

    useEffect(() => {
        GetTTSModelStatus().then((status) => {
            setModelStatus(status);
        }).catch((err) => console.error(err));

        const cancel = Events.On('tts:download-progress', (event) => {
            const status = event.data as any;
            setModelStatus(status);
        });
        return () => {
            cancel();
        };
    }, []);

    useEffect(() => {
        let cancelled = false;
        setAboutLoading(true);
        fetch(`${import.meta.env.BASE_URL}ABOUT.md`)
            .then((response) => {
                if (!response.ok) {
                    throw new Error(`Failed to load ABOUT.md (${response.status})`);
                }
                return response.text();
            })
            .then((markdown) => {
                if (cancelled) {
                    return;
                }
                setAboutMarkdown(markdown);
                setAboutError('');
            })
            .catch((err) => {
                if (cancelled) {
                    return;
                }
                setAboutError(String(err));
            })
            .finally(() => {
                if (!cancelled) {
                    setAboutLoading(false);
                }
            });
        return () => {
            cancelled = true;
        };
    }, []);

    useEffect(() => {
        const themeMode = generalSettings?.themeMode || 'auto';
        document.documentElement.dataset.theme = themeMode;
    }, [generalSettings?.themeMode]);

    useEffect(() => {
        if (detailMode === 'label' && selectedLabel) {
            setLabelForm({
                name: selectedLabel.name,
                description: selectedLabel.description,
                color: selectedLabel.color,
            });
        }
        if (detailMode === 'all') {
            setLabelForm(emptyLabelInput);
        }
    }, [detailMode, selectedLabel]);

    useEffect(() => {
        const timer = window.setInterval(() => {
            GetDashboard().then((nextStats) => {
                setStats({
                    ...nextStats,
                    typingHistory: nextStats.typingHistory || [],
                    topSnippets: nextStats.topSnippets || [],
                });
            }).catch((err) => setError(String(err)));
        }, 2500);
        return () => window.clearInterval(timer);
    }, []);

    useEffect(() => {
        const handleWindowShortcut = (event: globalThis.KeyboardEvent) => {
            if (event.metaKey && !event.altKey && !event.ctrlKey && !event.shiftKey && event.key.toLowerCase() === 'w') {
                event.preventDefault();
                event.stopPropagation();
                hideCurrentWindow();
                return;
            }
            if (windowMode === 'hud' && event.key === 'Escape') {
                event.preventDefault();
                event.stopPropagation();
                hideCurrentWindow();
            }
        };
        window.addEventListener('keydown', handleWindowShortcut, true);
        return () => window.removeEventListener('keydown', handleWindowShortcut, true);
    }, [aiRunning, windowMode]);

    useEffect(() => {
        if (!isHUD) {
            return;
        }
        const cancel = Events.On('ai:invoke', (event) => {
            const context = event.data as AIInvocationContext;
            setHasBeenInvoked(true);
            setWindowMode('hud');
            setActiveView('ai');
            setAIContext({
                kind: context?.kind || 'none',
                text: context?.text || '',
                filePath: context?.filePath || '',
                label: context?.label || 'No Context',
                sourceProcessId: context?.sourceProcessId || 0,
                appName: context?.appName || '',
                appBundleId: context?.appBundleId || '',
            });
            setAIPrompt('');
            setAIResult('');
            setAIReplacement('');
            window.setTimeout(() => {
                aiPromptRef.current?.focus();
                resizeAIPrompt();
            }, 30);
        });
        return cancel;
    }, []);

    useEffect(() => {
        if (isHUD) {
            return;
        }
        const cancel = Events.On('snippet:expanded', () => {
            playCompletionSound();
        });
        return cancel;
    }, []);

    useEffect(() => {
        resizeAIPrompt();
        resizeAIHUD();
    }, [aiPrompt, windowMode]);

    useEffect(() => {
        resizeAIHUD();
    }, [aiResult, aiReplacement, aiRunning, windowMode, aiElapsedMs]);

    useEffect(() => {
        if (!aiRunning) {
            setAIElapsedMs(0);
            return;
        }

        const startedAt = Date.now();
        setAIElapsedMs(0);
        const timer = window.setInterval(() => {
            setAIElapsedMs(Date.now() - startedAt);
        }, 250);
        return () => window.clearInterval(timer);
    }, [aiRunning]);

    useEffect(() => {
        if (isHUD) {
            return;
        }
        const cancel = Events.On('app:show-main', () => {
            setWindowMode('main');
            setActiveView('snippets');
            setAIResult('');
            setAIReplacement('');
        });
        return cancel;
    }, []);

    async function searchSnippets(value: string) {
        setQuery(value);
        try {
            await refresh(value, selectedLabelID);
        } catch (err) {
            setError(String(err));
        }
    }

    async function selectLabel(labelID: number) {
        setSelectedLabelID(labelID);
        setSelectedID(null);
        setDetailMode(labelID > 0 ? 'label' : 'all');
        try {
            await refresh(query, labelID);
        } catch (err) {
            setError(String(err));
        }
    }

    function openCreateModal() {
        setEditingSnippet(null);
        setForm({ ...emptyInput, labelId: selectedLabelID });
        setError('');
        setShortcutWarning('');
        setPastePreferenceTouched(false);
        setPasteWarning('');
        setIsModalOpen(true);
    }

    function openEditModal(snippet: Snippet) {
        setEditingSnippet(snippet);
        setForm({
            shortcut: snippet.shortcut,
            labelId: snippet.labelId,
            title: snippet.title,
            content: snippet.content,
            contentType: normalizeContentType(snippet.contentType),
            enabled: snippet.enabled,
            caseSensitive: snippet.caseSensitive,
            usePaste: snippet.usePaste,
            expandMode: snippet.expandMode,
        });
        setError('');
        setShortcutWarning('');
        setPastePreferenceTouched(false);
        setPasteWarning('');
        setIsModalOpen(true);
    }

    function updateSnippetContent(content: string) {
        if (!pastePreferenceTouched && !form.usePaste && shouldSuggestPaste(content)) {
            setForm({ ...form, content, usePaste: true });
            setPasteWarning('Paste is more stable for multiline or long snippets.');
            return;
        }
        setForm({ ...form, content });
    }

    function updateUsePaste(usePaste: boolean) {
        setPastePreferenceTouched(true);
        setPasteWarning('');
        setForm({ ...form, usePaste });
    }

    function insertSnippetToken(token: string) {
        const textarea = snippetContentRef.current;
        if (!textarea) {
            setForm((current) => ({ ...current, content: `${current.content}${token}` }));
            return;
        }

        const start = textarea.selectionStart ?? form.content.length;
        const end = textarea.selectionEnd ?? form.content.length;
        const nextContent = `${form.content.slice(0, start)}${token}${form.content.slice(end)}`;
        updateSnippetContent(nextContent);
        window.setTimeout(() => {
            textarea.focus();
            const nextCursor = start + token.length;
            textarea.setSelectionRange(nextCursor, nextCursor);
        }, 0);
    }

    async function submitSnippet(event: FormEvent) {
        event.preventDefault();
        setError('');
        if (hasUnsupportedShortcutCharacters(form.shortcut)) {
            setShortcutWarning('Shortcuts support only Roman letters, numbers, and symbols.');
            shortcutInputRef.current?.focus();
            return;
        }
        try {
            const saved = editingSnippet
                ? await UpdateSnippet(editingSnippet.id, form)
                : await CreateSnippet(form);
            setIsModalOpen(false);
            setSelectedID(saved.id);
            setDetailMode('snippet');
            await refresh();
        } catch (err) {
            if (isDuplicateShortcutError(err)) {
                setShortcutWarning('This shortcut is already in use.');
                shortcutInputRef.current?.focus();
                return;
            }
            setError(String(err));
        }
    }

    async function toggleSnippet(snippet: Snippet) {
        try {
            const updated = await ToggleSnippet(snippet.id, !snippet.enabled);
            setSnippets((current) => current.map((item) => item.id === updated.id ? updated : item));
            await refresh();
        } catch (err) {
            setError(String(err));
        }
    }

    async function createLabel() {
        const labelNumber = labels.length + 1;
        try {
            const label = await CreateLabel({
                name: `New Label ${labelNumber}`,
                description: '',
                color: '#153e75',
            });
            await selectLabel(label.id);
        } catch (err) {
            setError(String(err));
        }
    }

    async function saveSelectedLabel() {
        if (!selectedLabel) {
            return;
        }
        try {
            const label = await UpdateLabel(selectedLabel.id, labelForm);
            setLabels((current) => current.map((item) => item.id === label.id ? label : item));
            await refresh(query, selectedLabelID);
        } catch (err) {
            setError(String(err));
        }
    }

    async function removeSelectedLabel() {
        if (!selectedLabel) {
            return;
        }
        try {
            const confirmed = await ConfirmLabelDeletion(selectedLabel.name);
            if (!confirmed) {
                return;
            }
            await DeleteLabel(selectedLabel.id);
            await selectLabel(0);
        } catch (err) {
            setError(String(err));
        }
    }

    async function setCurrentLabelSnippetsEnabled(enabled: boolean) {
        const labelID = detailMode === 'label' && selectedLabel ? selectedLabel.id : 0;
        try {
            await SetLabelSnippetsEnabled(labelID, enabled);
            await refresh(query, selectedLabelID);
        } catch (err) {
            setError(String(err));
        }
    }

    async function assignSnippetToLabel(snippetID: number, labelID: number) {
        try {
            const updated = await AssignSnippetLabel(snippetID, labelID);
            setSnippets((current) => current.map((snippet) => snippet.id === updated.id ? updated : snippet));
            await refresh(query, selectedLabelID);
            setSelectedID(updated.id);
            setDetailMode('snippet');
        } catch (err) {
            setError(String(err));
        }
    }

    async function requestAccessibilityPermission() {
        setError('');
        try {
            const nextStatus = await RequestAccessibilityPermission();
            setPlatformStatus(nextStatus);
            await refresh();
        } catch (err) {
            setError(String(err));
        }
    }

    async function refreshPlatformStatus() {
        setError('');
        try {
            const nextStatus = await GetPlatformStatus();
            setPlatformStatus(nextStatus);
        } catch (err) {
            setError(String(err));
        }
    }

    async function hideCurrentWindow(cancelRunning = true) {
        setHasBeenInvoked(false);
        if (cancelRunning) {
            StopSpeaking().catch(() => {});
            if (aiRunning) {
                try {
                    await CancelAIRequest();
                } catch (err) {
                    setError(String(err));
                } finally {
                    setAIRunning(false);
                }
            }
        }
        Window.Hide();
    }

    function normalizeGeneralSettings(settings: { themeMode?: string; language?: string; typingTrendEnabled?: boolean; startAtLogin?: boolean; soundName?: string } = {}): GeneralSettings {
        const themeMode = settings.themeMode === 'light' || settings.themeMode === 'dark' ? settings.themeMode : 'auto';
        const soundName = settings.soundName && soundOptions.includes(settings.soundName)
            ? settings.soundName
            : noSoundName;
        return {
            themeMode,
            language: normalizeLanguage(settings.language),
            typingTrendEnabled: settings.typingTrendEnabled !== false,
            startAtLogin: settings.startAtLogin === true,
            soundName,
        };
    }

    function playCompletionSound(selectedSoundName = soundNameRef.current) {
        const soundName = selectedSoundName;
        if (soundName === noSoundName) {
            return;
        }
        const soundURL = soundURLs[soundName];
        if (!soundURL) {
            return;
        }
        const audio = soundName === soundNameRef.current && soundAudioRef.current
            ? soundAudioRef.current
            : new Audio(soundURL);
        audio.currentTime = 0;
        void audio.play().catch(() => undefined);
    }

    function updateGeneralSettings(patch: Partial<GeneralSettings>) {
        setGeneralSettings((current) => {
            if (!current) {
                return current;
            }
            return {
                ...current,
                ...patch,
            };
        });
    }

    function updateAndSaveGeneralSettings(patch: Partial<GeneralSettings>) {
        if (!generalSettings) {
            return;
        }
        const nextSettings = {
            ...generalSettings,
            ...patch,
        };
        setGeneralSettings(nextSettings);
        void saveGeneralSettings(nextSettings);
    }

    function updateSoundSetting(soundName: string) {
        updateAndSaveGeneralSettings({ soundName });
        playCompletionSound(soundName);
    }

    function normalizeAIPromptRule(rule: Partial<AIPromptRule> = {}): AIPromptRule {
        return {
            ...emptyPromptRule,
            ...rule,
            selectedTextPrompt: rule.selectedTextPrompt || '',
            noSelectionPrompt: rule.noSelectionPrompt || '',
        };
    }

    function normalizeAIPromptSettings(settings: any): AIPromptSettings {
        return {
            common: normalizeAIPromptRule(settings?.common),
            profiles: Array.isArray(settings?.profiles)
                ? settings.profiles.map((profile: any) => ({
                    ...normalizeAIPromptRule(profile),
                    id: profile.id || '',
                    appName: profile.appName || 'New App',
                    appBundleId: profile.appBundleId || '',
                    createdAt: profile.createdAt || '',
                    updatedAt: profile.updatedAt || '',
                }))
                : [],
        };
    }

    function normalizeAISettings(settings: any): AISettings {
        return {
            enabled: !!settings?.enabled,
            provider: settings?.provider || 'openai',
            endpoint: settings?.endpoint || 'http://localhost:1234',
            model: settings?.model || '',
            apiKey: settings?.apiKey || '',
            temperature: Number(settings?.temperature ?? 0),
            hotkey: settings?.hotkey || 'Cmd+Shift+Space',
            useSelectedText: settings?.useSelectedText ?? true,
            useSelectedFile: !!settings?.useSelectedFile,
            replaceSelectedText: settings?.replaceSelectedText ?? true,
            pasteReplacementBundleIds: Array.isArray(settings?.pasteReplacementBundleIds)
                ? settings.pasteReplacementBundleIds.filter((bundleId: unknown) => typeof bundleId === 'string')
                : ['com.apple.iWork.Keynote', 'com.apple.iWork.Pages', 'com.apple.iWork.Numbers'],
            ttsEnabled: !!settings?.ttsEnabled,
            ttsEngine: settings?.ttsEngine || 'os',
            ttsEndpoint: settings?.ttsEndpoint || 'http://localhost:7788',
            ttsVoice: settings?.ttsVoice || 'M1',
            ttsUseAiResponse: !!settings?.ttsUseAiResponse,
            ttsUseShortcut: !!settings?.ttsUseShortcut,
            ttsShortcut: settings?.ttsShortcut || 'Cmd+Shift+T',
            ttsSpeed: settings?.ttsSpeed || 1.05,
            ttsSteps: settings?.ttsSteps || 8,
        } as any;
    }

    function parseBundleIdList(value: string) {
        return Array.from(new Set(value
            .split(/[\n,]+/)
            .map((bundleId) => bundleId.trim())
            .filter(Boolean)));
    }

    function formatBundleIdList(bundleIds: string[] = []) {
        return bundleIds.join('\n');
    }

    async function saveGeneralSettings(nextSettings = generalSettings) {
        if (!nextSettings) {
            return;
        }
        setGeneralSaving(true);
        setError('');
        try {
            const saved = await SaveGeneralSettings(nextSettings);
            setGeneralSettings(normalizeGeneralSettings(saved));
        } catch (err) {
            setError(String(err));
        } finally {
            setGeneralSaving(false);
        }
    }

    async function saveAISettings(nextSettings = aiSettings) {
        if (!nextSettings) {
            return;
        }
        setAISaving(true);
        setError('');
        try {
            const saved = await SaveAISettings(nextSettings);
            setAISettings(normalizeAISettings(saved));
        } catch (err) {
            setError(String(err));
        } finally {
            setAISaving(false);
        }
    }

    async function saveCommonPromptRule() {
        if (!aiPromptSettings) {
            return;
        }
        setPromptSaving(true);
        setError('');
        try {
            const saved = await SaveCommonAIPromptRule(aiPromptSettings.common);
            setAIPromptSettings(normalizeAIPromptSettings(saved));
        } catch (err) {
            setError(String(err));
        } finally {
            setPromptSaving(false);
        }
    }

    async function createPromptProfile() {
        const current = platformStatus;
        setPromptSaving(true);
        setError('');
        try {
            const saved = await CreateAIPromptProfile({
                appName: current?.activeAppName || 'New App',
                appBundleId: current?.activeBundleId || '',
                ...emptyPromptRule,
            });
            const normalized = normalizeAIPromptSettings(saved);
            setAIPromptSettings(normalized);
            setSelectedPromptID((normalized.profiles || [])[(normalized.profiles || []).length - 1]?.id || 'common');
        } catch (err) {
            setError(String(err));
        } finally {
            setPromptSaving(false);
        }
    }

    async function savePromptProfile(profile: AIPromptProfile) {
        setPromptSaving(true);
        setError('');
        try {
            const saved = await UpdateAIPromptProfile(profile.id, {
                appName: profile.appName,
                appBundleId: profile.appBundleId,
                useSelectedText: profile.useSelectedText,
                runWithoutSelection: profile.runWithoutSelection,
                selectedTextPrompt: profile.selectedTextPrompt,
                noSelectionPrompt: profile.noSelectionPrompt,
            });
            setAIPromptSettings(normalizeAIPromptSettings(saved));
        } catch (err) {
            setError(String(err));
        } finally {
            setPromptSaving(false);
        }
    }

    async function deletePromptProfile(profile: AIPromptProfile) {
        setPromptSaving(true);
        setError('');
        try {
            const saved = await DeleteAIPromptProfile(profile.id);
            setAIPromptSettings(normalizeAIPromptSettings(saved));
            setSelectedPromptID('common');
        } catch (err) {
            setError(String(err));
        } finally {
            setPromptSaving(false);
        }
    }

    async function browsePromptProfileApp(profile: AIPromptProfile) {
        setError('');
        try {
            const appInfo = await BrowseAIPromptApp();
            if (!appInfo?.bundleId && !appInfo?.name) {
                return;
            }
            updatePromptProfile(profile.id, {
                appName: appInfo.name || profile.appName,
                appBundleId: appInfo.bundleId || profile.appBundleId,
            });
        } catch (err) {
            setError(String(err));
        }
    }

    async function browsePasteReplacementApp() {
        if (!aiSettings) {
            return;
        }
        setError('');
        try {
            const appInfo = await BrowseAIPromptApp();
            if (!appInfo?.bundleId) {
                return;
            }
            setAISettings({
                ...aiSettings,
                pasteReplacementBundleIds: parseBundleIdList([
                    ...(aiSettings.pasteReplacementBundleIds || []),
                    appInfo.bundleId,
                ].join('\n')),
            });
        } catch (err) {
            setError(String(err));
        }
    }

    function updateCommonPromptRule(patch: Partial<AIPromptRule>) {
        if (!aiPromptSettings) {
            return;
        }
        setAIPromptSettings({
            ...aiPromptSettings,
            common: { ...aiPromptSettings.common, ...patch },
        });
    }

    function updatePromptProfile(profileID: string, patch: Partial<AIPromptProfile>) {
        if (!aiPromptSettings) {
            return;
        }
        setAIPromptSettings({
            ...aiPromptSettings,
            profiles: (aiPromptSettings.profiles || []).map((profile) => (
                profile.id === profileID ? { ...profile, ...patch } : profile
            )),
        });
    }

    async function removeSnippet(snippet: Snippet) {
        const snippetLabel = snippet.title.trim() || snippet.shortcut;
        try {
            const confirmed = await ConfirmSnippetDeletion(snippetLabel);
            if (!confirmed) {
                return;
            }
            await DeleteSnippet(snippet.id);
            if (selectedID === snippet.id) {
                setSelectedID(null);
            }
            await refresh();
        } catch (err) {
            setError(String(err));
        }
    }

    function resizeAIPrompt() {
        const textarea = aiPromptRef.current;
        if (!textarea) {
            return;
        }
        textarea.style.height = 'auto';
        textarea.style.height = `${Math.min(textarea.scrollHeight, 118)}px`;
    }

    function resizeAIHUD() {
        if (windowMode !== 'hud' || !hasBeenInvoked) {
            return;
        }
        window.requestAnimationFrame(() => {
            const promptHeight = aiPromptRef.current?.offsetHeight ?? 34;
            const hasResult = Boolean(aiResult || aiReplacement);
            const statusHeight = aiRunning
                ? Math.min(document.querySelector<HTMLElement>('.ai-progress-status')?.scrollHeight ?? 0, 54)
                : 0;
            const resultHeight = hasResult
                ? Math.min(document.querySelector<HTMLElement>('.hud-result')?.scrollHeight ?? 0, 150)
                : 0;
            const nextHeight = Math.max(152, Math.min(360, 118 + promptHeight + statusHeight + resultHeight + (hasResult ? 18 : 0)));
            Window.SetSize(460, nextHeight);
            Window.Center();
        });
    }

    function updateAIPrompt(value: string) {
        setAIPrompt(value);
        window.requestAnimationFrame(resizeAIPrompt);
    }

    async function stopAIRequest() {
        try {
            await CancelAIRequest();
        } catch (err) {
            setError(String(err));
        } finally {
            setAIRunning(false);
        }
    }

    function waits(ms: number) {
        return new Promise((resolve) => window.setTimeout(resolve, ms));
    }

    async function runAIPrompt() {
        if (!aiPrompt.trim()) {
            return;
        }
        setAIRunning(true);
        setError('');
        setAIResult('');
        setAIReplacement('');
        try {
            StopSpeaking().catch(() => {});

            const result = await RunAIAssist({
                instruction: aiPrompt,
                contextKind: (aiContext.kind || 'none') as any,
                contextText: aiContext.text || '',
                filePath: '',
                appName: aiContext.appName || '',
                appBundleId: aiContext.appBundleId || '',
                customPrompt: '',
            });
            const isEdit = result.intent === 'edit' && !!result.replacement;
            if (
                aiSettings?.replaceSelectedText &&
                aiContext.sourceProcessId > 0 &&
                isEdit
            ) {
                const preferPasteReplacement = !!aiContext.appBundleId &&
                    (aiSettings.pasteReplacementBundleIds || []).includes(aiContext.appBundleId);
                if (preferPasteReplacement) {
                    await hideCurrentWindow(false);
                    await ActivateProcess(aiContext.sourceProcessId);
                    await waits(180);
                }
                await ReplaceSelectedText(aiContext.sourceProcessId, result.replacement);
                playCompletionSound();
                if (!preferPasteReplacement) {
                    await hideCurrentWindow(false);
                    await ActivateProcess(aiContext.sourceProcessId);
                }
                return;
            }
            setAIResult(isEdit ? '' : (result.supportReport || ''));
            setAIReplacement(isEdit ? result.replacement : '');
            playCompletionSound();

            // Speak if enabled
            // @ts-ignore
            if (aiSettings?.ttsEnabled && aiSettings?.ttsUseAiResponse) {
                const textToSpeak = isEdit ? result.replacement : (result.supportReport || '');
                if (textToSpeak) {
                    Speak(textToSpeak).catch(() => {});
                }
            }
        } catch (err) {
            setError(String(err));
        } finally {
            setAIRunning(false);
        }
    }

    async function submitAIPrompt(event: FormEvent) {
        event.preventDefault();
        await runAIPrompt();
    }

    function handleAIPromptKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
        if (event.key !== 'Enter' || event.shiftKey || event.nativeEvent.isComposing) {
            return;
        }
        event.preventDefault();
        if (aiRunning) {
            stopAIRequest();
            return;
        }
        runAIPrompt();
    }

    function captureHotkey(event: KeyboardEvent<HTMLButtonElement>) {
        if (!recordingHotkey || !aiSettings) {
            return;
        }
        event.preventDefault();
        event.stopPropagation();

        if (event.key === 'Escape') {
            setRecordingHotkey(false);
            return;
        }

        const nextHotkey = formatCapturedHotkey(event);
        if (!nextHotkey) {
            return;
        }
        setAISettings({ ...aiSettings, hotkey: nextHotkey });
        setRecordingHotkey(false);
    }

    function captureTtsHotkey(event: KeyboardEvent<HTMLButtonElement>) {
        if (!recordingTtsHotkey || !aiSettings) {
            return;
        }
        event.preventDefault();
        event.stopPropagation();

        if (event.key === 'Escape') {
            setRecordingTtsHotkey(false);
            return;
        }

        const nextHotkey = formatCapturedHotkey(event);
        if (!nextHotkey) {
            return;
        }
        setAISettings({
            ...aiSettings,
            // @ts-ignore
            ttsShortcut: nextHotkey,
        });
        setRecordingTtsHotkey(false);
    }

    return (
        <div className={`app-shell ${windowMode === 'hud' ? 'hud-shell' : ''}`}>
            {windowMode === 'main' && <aside className="sidebar">
                <div className="brand">
                    <img className="brand-mark" src={appIcon} alt="" />
                    <div>
                        <strong>DKST Text Flow</strong>
                        <span>{t('assistWithTextInput')}</span>
                    </div>
                </div>
                <nav className="nav">
                    <button className={activeView === 'snippets' ? 'active' : ''} onClick={() => setActiveView('snippets')}>
                        <span className="material-symbols-rounded" aria-hidden="true">snippet_folder</span>
                        <span>{t('snippets')}</span>
                    </button>
                    <button className={activeView === 'dashboard' ? 'active' : ''} onClick={() => setActiveView('dashboard')}>
                        <span className="material-symbols-rounded" aria-hidden="true">analytics</span>
                        <span>{t('dashboard')}</span>
                    </button>
                    <button className={activeView === 'aiPrompts' ? 'active' : ''} onClick={() => setActiveView('aiPrompts')}>
                        <span className="material-symbols-rounded" aria-hidden="true">chat_paste_go</span>
                        <span>{t('aiPrompt')}</span>
                    </button>
                    <button className={activeView === 'settings' ? 'active' : ''} onClick={() => setActiveView('settings')}>
                        <span className="material-symbols-rounded" aria-hidden="true">discover_tune</span>
                        <span>{t('settings')}</span>
                    </button>
                </nav>
                <div className="sidebar-bottom-actions">
                    <button className={`about-button ${activeView === 'about' ? 'active' : ''}`} type="button" onClick={() => setActiveView('about')}>
                        <span className="material-symbols-rounded" aria-hidden="true">info</span>
                        <span>{t('about')}</span>
                    </button>
                    <button className="quit-button" type="button" onClick={() => Application.Quit()}>
                        <span className="material-symbols-rounded" aria-hidden="true">power_settings_new</span>
                        <span>{t('quit')}</span>
                    </button>
                </div>
                <div className="status-tile">
                    <span className={platformStatus?.flowEngineRunning ? 'dot good' : 'dot idle'} />
                    <div>
                        <strong>{platformStatus?.flowEngineRunning ? t('flowActive') : t('flowPaused')}</strong>
                        <span>{platformStatus?.accessibilityTrusted ? t('accessibilityReady') : t('permissionPending')}</span>
                    </div>
                </div>
            </aside>}

            <main className="workspace">
                {windowMode === 'hud' ? (
                    <section className="ai-hud">
                        <div className="hud-header">
                            <div>
                                <h1>{t('aiAssist')}</h1>
                                <p>{aiContext.kind === 'selected_text' ? t('charactersSelected', { count: aiContext.text.length }) : t('noSelectedText')}</p>
                            </div>
                            <button
                                type="button"
                                onClick={() => hideCurrentWindow()}
                            >
                                {t('close')}
                            </button>
                        </div>
                        <form className="ai-prompt-form hud-prompt-form" onSubmit={submitAIPrompt}>
                            <textarea
                                ref={aiPromptRef}
                                value={aiPrompt}
                                onChange={(event) => updateAIPrompt(event.target.value)}
                                onKeyDown={handleAIPromptKeyDown}
                                placeholder={aiContext.kind === 'selected_text' ? t('aiHudSelectedPlaceholder') : t('aiHudPlaceholder')}
                                rows={1}
                            />
                            <div className="prompt-actions">
                                <button
                                    className="primary-button"
                                    type={aiRunning ? 'button' : 'submit'}
                                    disabled={!aiRunning && !aiPrompt.trim()}
                                    onClick={aiRunning ? stopAIRequest : undefined}
                                >
                                    {aiRunning && <span className="button-spinner" />}
                                    {aiRunning ? t('stop') : t('send')}
                                </button>
                            </div>
                        </form>
                        {aiRunning && <AIProgressStatus elapsedMs={aiElapsedMs} t={t} />}
                        {(aiResult || aiReplacement) && (
                            <div className="ai-result hud-result">
                                {aiResult && <p>{aiResult}</p>}
                                {aiReplacement && <pre>{aiReplacement}</pre>}
                            </div>
                        )}
                    </section>
                ) : activeView === 'snippets' ? (
                    <section className="content-grid">
                        <div className="panel labels-panel">
                            <div className="panel-header compact-header">
                                <div>
                                    <h1>{t('labels')}</h1>
                                    <p>{t('groups')}</p>
                                </div>
                                <button className="primary-button icon-button" onClick={createLabel} aria-label={t('addLabel')} title={t('addLabel')}>
                                    <span className="material-symbols-rounded" aria-hidden="true">add</span>
                                </button>
                            </div>
                            <div className="label-list">
                                {[allLabel, ...labels].map((label) => (
                                    <button
                                        key={label.id}
                                        className={`label-row ${selectedLabelID === label.id ? 'selected' : ''}`}
                                        onClick={() => selectLabel(label.id)}
                                        onDragOver={(event) => event.preventDefault()}
                                        onDrop={(event) => {
                                            const snippetID = Number(event.dataTransfer.getData('text/plain'));
                                            if (snippetID > 0) {
                                                assignSnippetToLabel(snippetID, label.id);
                                            }
                                        }}
                                    >
                                        <span className="label-color" style={{ backgroundColor: label.color }} />
                                        <span className="label-copy">
                                            <strong>{label.name}</strong>
                                            <span>{label.id === 0 ? t('everySnippet') : (label.description || t('noDescription'))}</span>
                                        </span>
                                    </button>
                                ))}
                            </div>
                        </div>
                        <div className="panel list-panel">
                            <div className="panel-header">
                                <div>
                                    <h1>{t('snippetLibrary')}</h1>
                                    <p>{selectedLabelID === 0 ? t('allSnippetsPeriod') : t('labelSnippets', { label: selectedLabel?.name ?? t('labelFallback') })}</p>
                                </div>
                                <button className="primary-button icon-button" onClick={openCreateModal} aria-label={t('newSnippet')} title={t('newSnippet')}>
                                    <span className="material-symbols-rounded" aria-hidden="true">add</span>
                                </button>
                            </div>
                            <input
                                className="search-input"
                                value={query}
                                placeholder={t('searchSnippets')}
                                onChange={(event) => searchSnippets(event.target.value)}
                            />
                            <div className="snippet-list">
                                {snippets.map((snippet) => (
                                    <button
                                        key={snippet.id}
                                        className={`snippet-row ${selectedSnippet?.id === snippet.id ? 'selected' : ''}`}
                                        style={{ '--snippet-label-color': labels.find((label) => label.id === snippet.labelId)?.color ?? '#667386' } as CSSProperties}
                                        draggable
                                        onDragStart={(event) => event.dataTransfer.setData('text/plain', String(snippet.id))}
                                        onClick={() => {
                                            setSelectedID(snippet.id);
                                            setDetailMode('snippet');
                                        }}
                                        onDoubleClick={() => openEditModal(snippet)}
                                    >
                                        <span className="shortcut">{snippet.shortcut}</span>
                                        <span className="snippet-title">{snippet.title}</span>
                                        <span className={snippet.enabled ? 'state enabled' : 'state disabled'}>{snippet.enabled ? t('on') : t('off')}</span>
                                    </button>
                                ))}
                            </div>
                        </div>

                        <div className="panel detail-panel">
                            {detailMode === 'label' && selectedLabel ? (
                                <>
                                    <div className="panel-header">
                                        <div>
                                            <h2>{selectedLabel.name}</h2>
                                            <p>{t('snippetsEnabledCount', { snippets: selectedLabel.snippetCount, enabled: selectedLabel.enabledCount })}</p>
                                        </div>
                                        <button className="danger icon-button" onClick={removeSelectedLabel} aria-label={t('deleteLabel')} title={t('deleteLabel')}>
                                            <span className="material-symbols-rounded" aria-hidden="true">delete_forever</span>
                                        </button>
                                        <div className="action-row">
                                            <button onClick={() => setCurrentLabelSnippetsEnabled(true)}>{t('enableAll')}</button>
                                            <button onClick={() => setCurrentLabelSnippetsEnabled(false)}>{t('disableAll')}</button>
                                        </div>
                                    </div>
                                    <div className="label-detail-form">
                                        <label>
                                            {t('name')}
                                            <input value={labelForm.name} onChange={(event) => setLabelForm({ ...labelForm, name: event.target.value })} />
                                        </label>
                                        <label>
                                            {t('color')}
                                            <input type="color" value={labelForm.color} onChange={(event) => setLabelForm({ ...labelForm, color: event.target.value })} />
                                        </label>
                                        <label className="label-description-field">
                                            {t('description')}
                                            <textarea value={labelForm.description} onChange={(event) => setLabelForm({ ...labelForm, description: event.target.value })} />
                                        </label>
                                    </div>
                                    <div className="modal-actions detail-actions">
                                        <button className="primary-button" onClick={saveSelectedLabel}>{t('saveLabel')}</button>
                                    </div>
                                </>
                            ) : detailMode === 'all' ? (
                                <>
                                    <div className="panel-header">
                                        <div>
                                            <h2>{t('allSnippets')}</h2>
                                            <p>{t('snippetsEnabledCount', { snippets: stats.snippetCount, enabled: stats.enabledCount })}</p>
                                        </div>
                                        <div className="action-row">
                                            <button onClick={() => setCurrentLabelSnippetsEnabled(true)}>{t('enableAll')}</button>
                                            <button onClick={() => setCurrentLabelSnippetsEnabled(false)}>{t('disableAll')}</button>
                                        </div>
                                    </div>
                                    <div className="empty-state">{t('selectLabelOrSnippet')}</div>
                                </>
                            ) : selectedSnippet ? (
                                <>
                                    <div className="panel-header">
                                        <div>
                                            <h2>{selectedSnippet.title}</h2>
                                            <p>{t('expandsAfter', { shortcut: selectedSnippet.shortcut, mode: t(selectedSnippet.expandMode === 'instant' ? 'instant' : 'delimiter') })}</p>
                                        </div>
                                        <button className="danger icon-button" onClick={() => removeSnippet(selectedSnippet)} aria-label={t('deleteSnippet')} title={t('deleteSnippet')}>
                                            <span className="material-symbols-rounded" aria-hidden="true">delete_forever</span>
                                        </button>
                                        <div className="action-row">
                                            <button onClick={() => toggleSnippet(selectedSnippet)}>{selectedSnippet.enabled ? t('disable') : t('enable')}</button>
                                            <button onClick={() => openEditModal(selectedSnippet)}>{t('edit')}</button>
                                        </div>
                                    </div>
                                    <pre className="snippet-preview">{selectedSnippet.content}</pre>
                                    <div className="meta-grid">
                                        <span>{t('label')}<strong>{labels.find((label) => label.id === selectedSnippet.labelId)?.name ?? t('all')}</strong></span>
                                        <span>{t('type')}<strong>{contentTypeLabel(selectedSnippet.contentType, t)}</strong></span>
                                        <span>{t('case')}<strong>{selectedSnippet.caseSensitive ? t('sensitive') : t('insensitive')}</strong></span>
                                        <span>{t('used')}<strong>{selectedSnippet.usageCount}</strong></span>
                                    </div>
                                </>
                            ) : (
                                <div className="empty-state">{t('createFirstSnippet')}</div>
                            )}
                        </div>
                    </section>
                ) : activeView === 'dashboard' ? (
                    <section className="panel dashboard-panel">
                        <div className="panel-header">
                            <div>
                                <h1>{t('dashboard')}</h1>
                                <p>{t('dashboardDescription')}</p>
                            </div>
                            <button onClick={() => refresh()}>{t('refresh')}</button>
                        </div>
                        <div className="dashboard">
                            <article>
                                <span>{t('totalExpansions')}</span>
                                <strong>{stats.totalExpansions}</strong>
                            </article>
                            <article>
                                <span>{t('today')}</span>
                                <strong>{stats.todayExpansions}</strong>
                            </article>
                            <article>
                                <span>{t('enabledSnippets')}</span>
                                <strong>{stats.enabledCount}/{stats.snippetCount}</strong>
                            </article>
                            <article>
                                <span>{t('todaysTyping')}</span>
                                <strong>{formatCount(stats.todayTypingCount)}</strong>
                            </article>
                            <article>
                                <span>{t('dailyAverage')}</span>
                                <strong>{formatCount(stats.averageDailyTyping)}</strong>
                            </article>
                        </div>
                        <TypingChart history={stats.typingHistory || []} enabled={generalSettings?.typingTrendEnabled !== false} t={t} />
                        <div className="top-list">
                            <h2>{t('topSnippets')}</h2>
                            {(stats.topSnippets || []).length === 0 ? (
                                <p>{t('noExpansionsYet')}</p>
                            ) : (stats.topSnippets || []).map((snippet) => (
                                <div key={snippet.id} className="top-row">
                                    <span className="shortcut">{snippet.shortcut}</span>
                                    <span className="top-title">{snippet.title}</span>
                                    <strong>{snippet.usageCount}</strong>
                                </div>
                            ))}
                        </div>
                    </section>
                ) : activeView === 'aiPrompts' ? (
                    <section className="content-grid ai-prompt-grid">
                        <div className="panel list-panel">
                            <div className="panel-header">
                                <div>
                                    <h1>{t('aiPrompt')}</h1>
                                    <p>{t('aiPromptDescription')}</p>
                                </div>
                                <button className="primary-button icon-button" onClick={createPromptProfile} disabled={promptSaving} aria-label={t('addAIPrompt')} title={t('addAIPrompt')}>
                                    <span className="material-symbols-rounded" aria-hidden="true">add</span>
                                </button>
                            </div>
                            <div className="snippet-list">
                                <button
                                    className={`snippet-row prompt-row ${selectedPromptID === 'common' ? 'selected' : ''}`}
                                    onClick={() => setSelectedPromptID('common')}
                                >
                                    <span className="shortcut">{t('common')}</span>
                                    <span className="snippet-title">{t('defaultBehavior')}</span>
                                    <span className="state enabled">{t('base')}</span>
                                </button>
                                {(aiPromptSettings?.profiles || []).map((profile) => (
                                    <button
                                        key={profile.id}
                                        className={`snippet-row prompt-row ${selectedPromptID === profile.id ? 'selected' : ''}`}
                                        onClick={() => setSelectedPromptID(profile.id)}
                                    >
                                        <span className="shortcut">{profile.appName}</span>
                                        <span className="snippet-title">{profile.appBundleId || t('noBundleId')}</span>
                                        <span className="state enabled">{t('app')}</span>
                                    </button>
                                ))}
                            </div>
                        </div>
                        <div className="panel detail-panel prompt-detail-panel">
                            {selectedPromptID === 'common' ? (
                                aiPromptSettings && (
                                    <>
                                        <div className="panel-header">
                                            <div>
                                                <h2>{t('common')}</h2>
                                                <p>{t('commonPromptDescription')}</p>
                                            </div>
                                            <button className="primary-button" onClick={saveCommonPromptRule} disabled={promptSaving}>
                                                {promptSaving ? t('saving') : t('save')}
                                            </button>
                                        </div>
                                        <PromptRuleEditor rule={aiPromptSettings.common} onChange={updateCommonPromptRule} t={t} />
                                    </>
                                )
                            ) : (
                                (() => {
                                    const profile = (aiPromptSettings?.profiles || []).find((item) => item.id === selectedPromptID);
                                    if (!profile) {
                                        return <div className="empty-state">{t('chooseAIPromptProfile')}</div>;
                                    }
                                    return (
                                        <>
                                            <div className="panel-header">
                                                <div>
                                                    <h2>{profile.appName}</h2>
                                                    <p>{profile.appBundleId || t('bundleIdNotSet')}</p>
                                                </div>
                                                <div className="action-row">
                                                    <button onClick={() => savePromptProfile(profile)} disabled={promptSaving}>{t('save')}</button>
                                                    <button className="danger icon-button" onClick={() => deletePromptProfile(profile)} disabled={promptSaving} aria-label={t('deleteSnippet')} title={t('deleteSnippet')}>
                                                        <span className="material-symbols-rounded" aria-hidden="true">delete_forever</span>
                                                    </button>
                                                </div>
                                            </div>
                                            <div className="prompt-app-fields">
                                                <label>
                                                    {t('appName')}
                                                    <input value={profile.appName} onChange={(event) => updatePromptProfile(profile.id, { appName: event.target.value })} />
                                                </label>
                                                <label>
                                                    {t('bundleId')}
                                                    <input value={profile.appBundleId} onChange={(event) => updatePromptProfile(profile.id, { appBundleId: event.target.value })} placeholder="com.apple.Terminal" />
                                                </label>
                                                <button type="button" onClick={() => browsePromptProfileApp(profile)}>{t('browse')}</button>
                                            </div>
                                            <PromptRuleEditor rule={profile} onChange={(patch) => updatePromptProfile(profile.id, patch)} t={t} />
                                        </>
                                    );
                                })()
                            )}
                        </div>
                    </section>
                ) : activeView === 'ai' ? (
                    <section className="panel ai-panel">
                        <div className="panel-header">
                            <div>
                                <h1>{t('aiAssistant')}</h1>
                                <p>{aiContext.kind === 'selected_text' ? t('aiPanelSelectedDescription') : t('aiPanelDefaultDescription')}</p>
                            </div>
                            <button onClick={() => setActiveView('settings')}>{t('settings')}</button>
                        </div>
                        <div className={`ai-context ${aiContext.kind !== 'none' ? 'active' : ''}`}>
                            <strong>{aiContext.label && aiContext.label !== 'No Context' ? aiContext.label : t('noContext')}</strong>
                            <span>
                                {aiContext.kind === 'selected_text'
                                    ? t('charactersCaptured', { count: aiContext.text.length })
                                    : t('noSelectedTextCaptured')}
                            </span>
                        </div>
                        <form className="ai-prompt-form" onSubmit={submitAIPrompt}>
                            <textarea
                                ref={aiPromptRef}
                                value={aiPrompt}
                                onChange={(event) => updateAIPrompt(event.target.value)}
                                onKeyDown={handleAIPromptKeyDown}
                                placeholder={aiContext.kind === 'selected_text' ? t('aiSelectedPlaceholder') : t('aiPlaceholder')}
                                rows={1}
                            />
                            <div className="prompt-actions">
                                <button
                                    className="primary-button"
                                    type={aiRunning ? 'button' : 'submit'}
                                    disabled={!aiRunning && !aiPrompt.trim()}
                                    onClick={aiRunning ? stopAIRequest : undefined}
                                >
                                    {aiRunning && <span className="button-spinner" />}
                                    {aiRunning ? t('stop') : t('send')}
                                </button>
                            </div>
                        </form>
                        {aiRunning && <AIProgressStatus elapsedMs={aiElapsedMs} t={t} />}
                        {(aiResult || aiReplacement) && (
                            <div className="ai-result">
                                {aiResult && <p>{aiResult}</p>}
                                {aiReplacement && <pre>{aiReplacement}</pre>}
                            </div>
                        )}
                    </section>
                ) : activeView === 'about' ? (
                    <section className="panel about-panel">
                        {aboutLoading ? (
                            <div className="empty-state">{t('loadingAbout')}</div>
                        ) : aboutError ? (
                            <div className="empty-state">{aboutError}</div>
                        ) : (
                            <ReactMarkdown
                                className="markdown-body"
                                remarkPlugins={[remarkGfm]}
                                rehypePlugins={[rehypeRaw]}
                                components={aboutMarkdownComponents}
                            >
                                {aboutMarkdown}
                            </ReactMarkdown>
                        )}
                    </section>
                ) : (
                    <section className="panel settings-panel">
                        <div className="panel-header">
                            <div>
                                <h1>{t('settings')}</h1>
                                <p>{t('settingsDescription')}</p>
                            </div>
                            <div className="action-row">
                                <button onClick={refreshPlatformStatus}>{t('refresh')}</button>
                                <button
                                    className={platformStatus?.accessibilityTrusted ? undefined : 'primary-button'}
                                    onClick={requestAccessibilityPermission}
                                >
                                    {t('requestPermission')}
                                </button>
                            </div>
                        </div>
                        <div className="settings-list">
                            <div className={platformStatus?.accessibilityTrusted ? 'settings-status success' : 'settings-status danger'}>
                                <span>{t('accessibilityPermission')}</span>
                                <strong>{platformStatus?.accessibilityTrusted ? t('granted') : t('required')}</strong>
                            </div>
                            <div>
                                <span>{t('secureInput')}</span>
                                <strong>{platformStatus?.secureInputActive ? t('active') : t('notDetected')}</strong>
                            </div>
                            <div>
                                <span>{t('aiActiveStatus')}</span>
                                <strong>{aiSettings?.enabled ? t('enabled') : t('disabled')}</strong>
                            </div>
                        </div>
                        {aiSettings && (
                            <>
                                {generalSettings && (
                                    <form
                                        className="settings-section"
                                        onSubmit={(event) => {
                                            event.preventDefault();
                                            saveGeneralSettings();
                                        }}
                                    >
                                        <div className="panel-header compact">
                                            <div>
                                                <h2>{t('general')}</h2>
                                                <p>{t('generalDescription')}</p>
                                            </div>
                                            {/* <button className="primary-button" type="submit" disabled={generalSaving}>
                                                {generalSaving ? t('saving') : t('save')}
                                            </button> */}
                                        </div>
                                        <div className="settings-form-grid">
                                            <label>
                                                {t('language')}
                                                <select
                                                    value={generalSettings.language}
                                                    onChange={(event) => updateGeneralSettings({
                                                        language: normalizeLanguage(event.target.value),
                                                    })}
                                                >
                                                    {languageOptions.map((option) => (
                                                        <option key={option.value} value={option.value}>{option.label}</option>
                                                    ))}
                                                </select>
                                            </label>
                                            <label>
                                                {t('appearance')}
                                                <select
                                                    value={generalSettings.themeMode}
                                                    onChange={(event) => updateGeneralSettings({
                                                        themeMode: event.target.value as GeneralSettings['themeMode'],
                                                    })}
                                                >
                                                    <option value="auto">{t('auto')}</option>
                                                    <option value="light">{t('light')}</option>
                                                    <option value="dark">{t('dark')}</option>
                                                </select>
                                            </label>
                                            <label>
                                                {t('typingTrend')}
                                                <select
                                                    value={generalSettings.typingTrendEnabled ? 'on' : 'off'}
                                                    onChange={(event) => updateGeneralSettings({
                                                        typingTrendEnabled: event.target.value === 'on',
                                                    })}
                                                >
                                                    <option value="on">{t('on')}</option>
                                                    <option value="off">{t('off')}</option>
                                                </select>
                                            </label>
                                            <label>
                                                {t('sound')}
                                                <select
                                                    value={generalSettings.soundName}
                                                    onChange={(event) => updateSoundSetting(event.target.value)}
                                                >
                                                    {soundOptions.map((soundName) => (
                                                        <option key={soundName} value={soundName}>{soundName}</option>
                                                    ))}
                                                </select>
                                            </label>
                                            <label className="checkbox-setting">
                                                <span>{t('startAtLogin')}</span>
                                                <input
                                                    type="checkbox"
                                                    checked={generalSettings.startAtLogin}
                                                    disabled={generalSaving}
                                                    onChange={(event) => updateAndSaveGeneralSettings({
                                                        startAtLogin: event.target.checked,
                                                    })}
                                                />
                                            </label>
                                        </div>
                                    </form>
                                )}
                                <form
                                    className="settings-section ai-settings"
                                    onSubmit={(event) => {
                                        event.preventDefault();
                                        saveAISettings();
                                    }}
                                >
                                    <div className="panel-header compact">
                                        <div>
                                            <h2>{t('aiAssistant')}</h2>
                                            <p>{t('aiSettingsDescription')}</p>
                                        </div>
                                        <button className="primary-button" type="submit" disabled={aiSaving}>
                                            {aiSaving ? t('saving') : t('save')}
                                        </button>
                                    </div>
                                    <div className="check-row">
                                        <label>
                                            <input
                                                type="checkbox"
                                                checked={aiSettings.enabled}
                                                onChange={(event) => setAISettings({ ...aiSettings, enabled: event.target.checked })}
                                            /> {t('enableAiAssistant')}
                                        </label>
                                        <label>
                                            <input
                                                type="checkbox"
                                                checked={aiSettings.useSelectedText}
                                                onChange={(event) => setAISettings({ ...aiSettings, useSelectedText: event.target.checked })}
                                            /> {t('useSelectedText')}
                                        </label>
                                        <label>
                                            <input
                                                type="checkbox"
                                                checked={aiSettings.replaceSelectedText}
                                                onChange={(event) => setAISettings({ ...aiSettings, replaceSelectedText: event.target.checked })}
                                            /> {t('replaceSelectedText')}
                                        </label>
                                    </div>
                                    <div className="settings-form-grid">
                                        <label>
                                            {t('provider')}
                                            <select
                                                value={aiSettings.provider}
                                                onChange={(event) => setAISettings({ ...aiSettings, provider: event.target.value })}
                                            >
                                                <option value="openai">{t('openaiCompatible')}</option>
                                                <option value="lmstudio">{t('lmStudioCompatible')}</option>
                                            </select>
                                        </label>
                                        <label>
                                            {t('promptHotkey')}
                                            <button
                                                className={`hotkey-capture ${recordingHotkey ? 'recording' : ''}`}
                                                type="button"
                                                onClick={(event) => {
                                                    setRecordingHotkey(true);
                                                    event.currentTarget.focus();
                                                }}
                                                onBlur={() => setRecordingHotkey(false)}
                                                onKeyDown={captureHotkey}
                                            >
                                                {recordingHotkey ? t('pressShortcut') : (aiSettings.hotkey || t('recordShortcut'))}
                                            </button>
                                        </label>
                                    </div>
                                    <div className="settings-form-grid">
                                        <label>
                                            {t('endpointOrHostPort')}
                                            <input
                                                value={aiSettings.endpoint}
                                                onChange={(event) => setAISettings({ ...aiSettings, endpoint: event.target.value })}
                                                placeholder="http://localhost:1234"
                                            />
                                        </label>
                                        <label>
                                            {t('model')}
                                            <input
                                                value={aiSettings.model}
                                                onChange={(event) => setAISettings({ ...aiSettings, model: event.target.value })}
                                                placeholder={t('modelPlaceholder')}
                                            />
                                        </label>
                                    </div>
                                    <div className="settings-form-grid">
                                        <label>
                                            {t('apiKey')}
                                            <input
                                                value={aiSettings.apiKey}
                                                onChange={(event) => setAISettings({ ...aiSettings, apiKey: event.target.value })}
                                                placeholder={t('optional')}
                                                type="password"
                                            />
                                        </label>
                                        <label>
                                            {t('temperature')}
                                            <input
                                                value={aiSettings.temperature}
                                                min={0}
                                                max={2}
                                                step={0.1}
                                                type="number"
                                                onChange={(event) => setAISettings({ ...aiSettings, temperature: Number(event.target.value) })}
                                            />
                                        </label>
                                    </div>
                                    <label className="wide-setting">
                                        {t('pasteReplacementBundleIds')}
                                        <div className="bundle-list-control">
                                            <textarea
                                                value={formatBundleIdList(aiSettings.pasteReplacementBundleIds || [])}
                                                onChange={(event) => setAISettings({
                                                    ...aiSettings,
                                                    pasteReplacementBundleIds: parseBundleIdList(event.target.value),
                                                })}
                                                placeholder="com.apple.iWork.Keynote&#10;com.apple.iWork.Pages&#10;com.apple.iWork.Numbers"
                                                rows={3}
                                            />
                                            <button type="button" onClick={browsePasteReplacementApp}>{t('browse')}</button>
                                        </div>
                                        <span className="field-hint">{t('pasteReplacementBundleIdsHint')}</span>
                                    </label>

                                    <hr className="settings-divider" />

                                    <div className="settings-section-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                        <div>
                                            <h3>{t('ttsSettings')}</h3>
                                            <p>{t('ttsSettingsDescription')}</p>
                                        </div>
                                        <button
                                            className="tts-test-playback-btn"
                                            type="button"
                                            onClick={() => {
                                                TestSpeak("Hello! 안녕하세요. DKST Text Flow TTS 테스트입니다.", aiSettings).catch(err => alert(err));
                                            }}
                                        >
                                            {t('testPlayback')}
                                        </button>
                                    </div>

                                    <div className="check-row">
                                        <label>
                                            <input
                                                type="checkbox"
                                                // @ts-ignore
                                                checked={!!aiSettings.ttsEnabled}
                                                // @ts-ignore
                                                onChange={(event) => setAISettings({ ...aiSettings, ttsEnabled: event.target.checked })}
                                            /> {t('enableTts')}
                                        </label>
                                        <label>
                                            <input
                                                type="checkbox"
                                                // @ts-ignore
                                                checked={!!aiSettings.ttsUseAiResponse}
                                                // @ts-ignore
                                                onChange={(event) => setAISettings({ ...aiSettings, ttsUseAiResponse: event.target.checked })}
                                            /> {t('ttsUseAiResponse')}
                                        </label>
                                        <label>
                                            <input
                                                type="checkbox"
                                                // @ts-ignore
                                                checked={!!aiSettings.ttsUseShortcut}
                                                // @ts-ignore
                                                onChange={(event) => setAISettings({ ...aiSettings, ttsUseShortcut: event.target.checked })}
                                            /> {t('ttsUseShortcut')}
                                        </label>
                                    </div>

                                    <div className="settings-form-grid">
                                        <label>
                                            {t('ttsEngine')}
                                            <select
                                                // @ts-ignore
                                                value={aiSettings.ttsEngine || 'os'}
                                                // @ts-ignore
                                                onChange={(event) => setAISettings({ ...aiSettings, ttsEngine: event.target.value })}
                                            >
                                                <option value="os">{t('osTts')}</option>
                                                <option value="supertonic3">{t('supertonic3')}</option>
                                            </select>
                                        </label>
                                        <label>
                                            {t('ttsShortcut')}
                                            <button
                                                className={`hotkey-capture ${recordingTtsHotkey ? 'recording' : ''}`}
                                                type="button"
                                                onClick={(event) => {
                                                    setRecordingTtsHotkey(true);
                                                    event.currentTarget.focus();
                                                }}
                                                onBlur={() => setRecordingTtsHotkey(false)}
                                                onKeyDown={captureTtsHotkey}
                                            >
                                                {/* @ts-ignore */}
                                                {recordingTtsHotkey ? t('pressShortcut') : (aiSettings.ttsShortcut || t('recordShortcut'))}
                                            </button>
                                        </label>
                                    </div>

                                    {/* @ts-ignore */}
                                    {aiSettings.ttsEngine === 'supertonic3' && (
                                        <>
                                            <div className="settings-form-grid">
                                                <label>
                                                    {t('supertonicVoiceStyle')}
                                                    <select
                                                        // @ts-ignore
                                                        value={aiSettings.ttsVoice || 'M1'}
                                                        // @ts-ignore
                                                        onChange={(event) => setAISettings({ ...aiSettings, ttsVoice: event.target.value })}
                                                    >
                                                        <option value="M1">{t('voiceM1')}</option>
                                                        <option value="M2">{t('voiceM2')}</option>
                                                        <option value="M3">{t('voiceM3')}</option>
                                                        <option value="M4">{t('voiceM4')}</option>
                                                        <option value="M5">{t('voiceM5')}</option>
                                                        <option value="F1">{t('voiceF1')}</option>
                                                        <option value="F2">{t('voiceF2')}</option>
                                                        <option value="F3">{t('voiceF3')}</option>
                                                        <option value="F4">{t('voiceF4')}</option>
                                                        <option value="F5">{t('voiceF5')}</option>
                                                    </select>
                                                </label>
                                            </div>
                                            <div className="settings-form-grid" style={{ marginTop: '10px' }}>
                                                <label>
                                                    {t('supertonicSpeed')} ({aiSettings.ttsSpeed || 1.05})
                                                    <input
                                                        type="range"
                                                        min="0.7"
                                                        max="2.0"
                                                        step="0.05"
                                                        // @ts-ignore
                                                        value={aiSettings.ttsSpeed || 1.05}
                                                        // @ts-ignore
                                                        onChange={(event) => setAISettings({ ...aiSettings, ttsSpeed: parseFloat(event.target.value) })}
                                                    />
                                                </label>
                                                <label>
                                                    {t('supertonicSteps')} ({aiSettings.ttsSteps || 8})
                                                    <input
                                                        type="range"
                                                        min="5"
                                                        max="12"
                                                        step="1"
                                                        // @ts-ignore
                                                        value={aiSettings.ttsSteps || 8}
                                                        // @ts-ignore
                                                        onChange={(event) => setAISettings({ ...aiSettings, ttsSteps: parseInt(event.target.value) })}
                                                    />
                                                </label>
                                            </div>
                                            <div className="tts-model-status-section">
                                                <div className="tts-model-status-title">
                                                    {modelStatus.isDownloaded ? t('ttsModelReady') : (
                                                        modelStatus.status === 'error' ? t('ttsModelDownloadFailed') : t('ttsModelNotReady')
                                                    )}
                                                    {modelStatus.status === 'error' && modelStatus.error && (
                                                        <div style={{ color: '#ff4d4f', fontSize: '11px', marginTop: '4px', fontWeight: 'normal', wordBreak: 'break-all' }}>
                                                            {modelStatus.error}
                                                        </div>
                                                    )}
                                                </div>
                                                {modelStatus.status === 'downloading' && (
                                                    <>
                                                        <div className="tts-model-status-desc">
                                                            {t('downloadingTtsModel', { progress: Math.round(modelStatus.progress) })}
                                                            {modelStatus.currentFile && ` (${modelStatus.currentFile})`}
                                                        </div>
                                                        <div className="tts-model-progress-bar">
                                                            <div className="tts-model-progress-fill" style={{ width: `${modelStatus.progress}%` }}></div>
                                                        </div>
                                                        <button
                                                            className="tts-model-download-btn cancel"
                                                            type="button"
                                                            onClick={() => CancelTTSModelDownload()}
                                                        >
                                                            {t('cancel')}
                                                        </button>
                                                    </>
                                                )}
                                                {modelStatus.status !== 'downloading' && (
                                                    <button
                                                        className="tts-model-download-btn"
                                                        type="button"
                                                        onClick={() => {
                                                            StartTTSModelDownload().catch(err => alert(err));
                                                        }}
                                                    >
                                                        {t('downloadTtsModel')}
                                                    </button>
                                                )}
                                            </div>
                                        </>
                                    )}
                                </form>
                            </>
                        )}
                    </section>
                )}

                {error && <div className="toast">{error}</div>}
            </main>

            {isModalOpen && (
                <div className="modal-backdrop">
                    <form className="modal snippet-modal" onSubmit={submitSnippet}>
                        <div className="panel-header">
                            <div>
                                <h2>{editingSnippet ? t('editSnippet') : t('newSnippetTitle')}</h2>
                                <p>{t('snippetHelp')}</p>
                            </div>
                            <div className="modal-actions header-actions">
                                <button type="button" onClick={() => setIsModalOpen(false)}>{t('cancel')}</button>
                                <button className="primary-button" type="submit">{t('save')}</button>
                            </div>
                        </div>
                        <div className="snippet-main-fields">
                            <label className={`shortcut-field ${shortcutWarning ? 'invalid' : ''}`}>
                                <span>{t('shortcut')}</span>
                                <span className="shortcut-input-wrap">
                                    <input
                                        ref={shortcutInputRef}
                                        value={form.shortcut}
                                        onChange={(event) => {
                                            setForm({ ...form, shortcut: event.target.value });
                                            setShortcutWarning('');
                                        }}
                                        aria-invalid={Boolean(shortcutWarning)}
                                        aria-describedby={shortcutWarning ? 'shortcut-warning' : undefined}
                                        placeholder={t('shortcutPlaceholder')}
                                    />
                                    {shortcutWarning && (
                                        <span className="field-tooltip" id="shortcut-warning" role="alert">
                                            {shortcutWarning}
                                        </span>
                                    )}
                                </span>
                            </label>
                            <label>
                                {t('label')}
                                <select value={form.labelId} onChange={(event) => setForm({ ...form, labelId: Number(event.target.value) })}>
                                    <option value={0}>{t('all')}</option>
                                    {labels.map((label) => (
                                        <option key={label.id} value={label.id}>{label.name}</option>
                                    ))}
                                </select>
                            </label>
                            <label className="title-field">
                                {t('title')}
                                <input value={form.title} onChange={(event) => setForm({ ...form, title: event.target.value })} placeholder={t('titlePlaceholder')} />
                            </label>
                        </div>
                        <label className="content-field">
                            {t('content')}
                            <textarea
                                ref={snippetContentRef}
                                value={form.content}
                                onChange={(event) => updateSnippetContent(event.target.value)}
                            />
                        </label>
                        <div className="snippet-token-row modal-token-row">
                            {snippetTokens.map((token) => (
                                <button key={token.value} type="button" onClick={() => insertSnippetToken(token.value)}>
                                    {t(token.labelKey)}
                                </button>
                            ))}
                        </div>
                        <div className="form-grid">
                            <label>
                                {t('expandMode')}
                                <select value={form.expandMode} onChange={(event) => setForm({ ...form, expandMode: event.target.value })}>
                                    <option value="delimiter">{t('delimiter')}</option>
                                    <option value="instant">{t('instant')}</option>
                                </select>
                            </label>
                            <label>
                                {t('contentType')}
                                <select value={form.contentType} onChange={(event) => setForm({ ...form, contentType: event.target.value })}>
                                    {contentTypeOptions.map((option) => (
                                        <option key={option.value} value={option.value}>{t(option.labelKey)}</option>
                                    ))}
                                </select>
                            </label>
                        </div>
                        <div className="check-row">
                            <label><input type="checkbox" checked={form.enabled} onChange={(event) => setForm({ ...form, enabled: event.target.checked })} /> {t('enabled')}</label>
                            <label className="paste-option">
                                <input type="checkbox" checked={form.usePaste} onChange={(event) => updateUsePaste(event.target.checked)} /> {t('paste')}
                                {pasteWarning && (
                                    <span className="field-tooltip paste-tooltip" role="status">
                                        {pasteWarning}
                                    </span>
                                )}
                            </label>
                            <label><input type="checkbox" checked={form.caseSensitive} onChange={(event) => setForm({ ...form, caseSensitive: event.target.checked })} /> {t('caseSensitive')}</label>
                        </div>
                    </form>
                </div>
            )}
        </div>
    );
}

function AIProgressStatus({ elapsedMs, t }: { elapsedMs: number; t: Translator }) {
    return (
        <div className="ai-progress-status" role="status" aria-live="polite">
            <div className="ai-progress-row">
                <span className="ai-pulse-dot" />
                <span>{aiStatusLabel(elapsedMs, t)}</span>
                <span className="ai-elapsed">{Math.floor(elapsedMs / 1000)}s</span>
            </div>
            <div className="ai-progress-track">
                <span />
            </div>
        </div>
    );
}

function TypingChart({ history, enabled, t }: { history: DailyTypingStat[]; enabled: boolean; t: Translator }) {
    const maxCount = Math.max(1, ...history.map((day) => day.count));
    return (
        <div className="typing-chart">
            <div className="typing-chart-header">
                <div>
                    <h2>{t('typingTrend')}</h2>
                    <p>{t('typingTrendDescription')}</p>
                </div>
            </div>
            <div className="typing-bars" aria-label={t('typingTrendDescription')}>
                {history.map((day) => {
                    const height = Math.max(3, Math.round((day.count / maxCount) * 100));
                    return (
                        <span
                            key={day.date}
                            className="typing-bar"
                            style={{ height: `${height}%` }}
                            title={`${day.date}: ${formatCount(day.count)}`}
                            aria-label={`${day.date}: ${formatCount(day.count)}`}
                        />
                    );
                })}
                {!enabled && (
                    <div className="typing-disabled-overlay">
                        <strong>{t('typingTrendOff')}</strong>
                        <span>{t('typingTrendOffDescription')}</span>
                    </div>
                )}
            </div>
        </div>
    );
}

function PromptRuleEditor({ rule, onChange, t }: { rule: AIPromptRule; onChange: (patch: Partial<AIPromptRule>) => void; t: Translator }) {
    return (
        <div className="prompt-rule-editor">
            <div className="check-row">
                <label>
                    <input
                        type="checkbox"
                        checked={rule.useSelectedText}
                        onChange={(event) => onChange({ useSelectedText: event.target.checked })}
                    /> {t('useSelectedText')}
                </label>
                <label>
                    <input
                        type="checkbox"
                        checked={rule.runWithoutSelection}
                        onChange={(event) => onChange({ runWithoutSelection: event.target.checked })}
                    /> {t('runWithoutSelection')}
                </label>
            </div>
            <label>
                {t('promptWhenTextSelected')}
                <textarea
                    value={rule.selectedTextPrompt}
                    onChange={(event) => onChange({ selectedTextPrompt: event.target.value })}
                    placeholder={t('selectedTextPromptPlaceholder')}
                    rows={7}
                />
            </label>
            <label>
                {t('promptWhenNoTextSelected')}
                <textarea
                    value={rule.noSelectionPrompt}
                    onChange={(event) => onChange({ noSelectionPrompt: event.target.value })}
                    placeholder={t('noTextPromptPlaceholder')}
                    rows={7}
                />
            </label>
        </div>
    );
}

export default App;
