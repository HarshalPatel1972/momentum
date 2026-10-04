import { useCallback, useEffect, useRef, useState } from 'react';
import {
    EventsOn, ScreenGetAll, WindowGetPosition, WindowGetSize, WindowSetAlwaysOnTop,
    WindowSetMinSize, WindowSetPosition, WindowSetSize,
} from '../wailsjs/runtime';
import { Activity, api, AppState } from './lib';
import { Page, PAGES, Rail, StatusBar, TitleBar } from './components/Chrome';
import Welcome from './components/Welcome';
import Setup from './components/Setup';
import Home from './components/Home';
import Mini from './components/Mini';
import { AboutPage, IdesPage, LinkPage, LogPage, SettingsPage } from './components/Pages';

type Mode = 'loading' | 'welcome' | 'setup' | 'main';

const MINI = { w: 330, h: 268 };
const FULL_MIN = { w: 860, h: 580 };

export default function App() {
    const [mode, setMode] = useState<Mode>('loading');
    const [page, setPage] = useState<Page>('home');
    const [state, setState] = useState<AppState | null>(null);
    const [activity, setActivity] = useState<Activity[]>([]);
    const [mini, setMini] = useState(false);
    const [toastMsg, setToastMsg] = useState('');
    const toastTimer = useRef<number>();
    const saved = useRef<{ w: number; h: number; x: number; y: number } | null>(null);
    const popped = useRef(false); // the mini pager popped up by itself for a page

    const refresh = useCallback(async () => {
        const [s, a] = await Promise.all([api.state(), api.activity()]);
        setState(s);
        setActivity(a || []);
        return s;
    }, []);

    const toast = useCallback((msg: string) => {
        setToastMsg(msg);
        window.clearTimeout(toastTimer.current);
        toastTimer.current = window.setTimeout(() => setToastMsg(''), 3200);
    }, []);

    // ----- mini pager: shrink the window into an always-on-top widget in the corner -----
    const enterMini = useCallback(async () => {
        const [size, pos] = await Promise.all([WindowGetSize(), WindowGetPosition()]);
        saved.current = { w: size.w, h: size.h, x: pos.x, y: pos.y };
        WindowSetMinSize(MINI.w, MINI.h);
        WindowSetSize(MINI.w, MINI.h);
        const screens = await ScreenGetAll();
        const s = screens.find(x => x.isCurrent) || screens.find(x => x.isPrimary) || screens[0];
        if (s) WindowSetPosition(s.width - MINI.w - 18, s.height - MINI.h - 64);
        WindowSetAlwaysOnTop(true);
        setMini(true);
    }, []);

    const exitMini = useCallback(async () => {
        WindowSetAlwaysOnTop(false);
        WindowSetMinSize(FULL_MIN.w, FULL_MIN.h);
        const p = saved.current;
        WindowSetSize(p?.w || 1020, p?.h || 680);
        if (p) WindowSetPosition(p.x, p.y);
        popped.current = false;
        setMini(false);
    }, []);

    const toggleMini = useCallback(() => (mini ? exitMini() : enterMini()), [mini, enterMini, exitMini]);

    useEffect(() => {
        // "#setup-2", "#welcome" or "#<page>" opens a screen directly (previews/screenshots).
        const hash = window.location.hash.slice(1);
        refresh().then(s => {
            if (hash === 'welcome' || hash.startsWith('setup')) return setMode(hash === 'welcome' ? 'welcome' : 'setup');
            if ([...PAGES.map(p => p.id), 'about'].includes(hash as Page)) setPage(hash as Page);
            setMode(s.configured ? 'main' : 'welcome');
            if (hash === 'mini') setMini(true);
        });
        const offs = [
            EventsOn('activity', () => api.activity().then(a => setActivity(a || []))),
            EventsOn('state', () => refresh()),
        ];
        const poll = window.setInterval(refresh, 8000);
        return () => { offs.forEach(off => off()); window.clearInterval(poll); };
    }, [refresh]);

    // A page arrived while the window was closed: pop up the mini pager.
    useEffect(() => EventsOn('popup', async () => {
        api.show();
        if (!mini) await enterMini();
        popped.current = true;
    }), [mini, enterMini]);

    // Keyboard: Ctrl+1–4 / Ctrl+, switch views, Ctrl+M toggles the mini pager.
    useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if (!e.ctrlKey || mode !== 'main') return;
            const p = PAGES.find(x => x.key === e.key);
            if (p) { e.preventDefault(); if (mini) exitMini(); setPage(p.id); }
            if (e.key.toLowerCase() === 'm') { e.preventDefault(); toggleMini(); }
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [mode, mini, toggleMini, exitMini]);

    const waiting = activity.filter(a => a.state === 'waiting');

    if (mode === 'loading' || !state) return <div style={{ height: '100%', background: 'var(--body)' }} />;

    if (mini) {
        return (
            <Mini
                page={waiting[0]}
                onExpand={exitMini}
                onClose={async () => { await exitMini(); api.hide(); }}
                onAnswered={() => {
                    // Popped up just for this page: tuck back into the tray once answered.
                    if (popped.current) setTimeout(async () => { await exitMini(); api.hide(); }, 2200);
                }}
            />
        );
    }

    return (
        <div className="window">
            <TitleBar live={waiting.length > 0} />
            {mode === 'welcome' && <Welcome onStart={() => setMode('setup')} />}
            {mode === 'setup' && (
                <Setup
                    initialStep={Number(window.location.hash.split('-')[1] || 1) - 1}
                    onBack={() => setMode('welcome')}
                    onFinish={() => { refresh(); setMode('main'); setPage('home'); }}
                />
            )}
            {mode === 'main' && (
                <div className="frame">
                    <Rail page={page} go={setPage} waiting={waiting.length} />
                    <div className="view" key={page}>
                        {page === 'home' && <Home state={state} activity={activity} go={p => setPage(p)} toast={toast} refresh={refresh} />}
                        {page === 'log' && <LogPage activity={activity} toast={toast} />}
                        {page === 'ides' && <IdesPage refresh={refresh} toast={toast} />}
                        {page === 'link' && <LinkPage state={state} refresh={refresh} toast={toast} />}
                        {page === 'settings' && <SettingsPage state={state} refresh={refresh} toast={toast} />}
                        {page === 'about' && <AboutPage state={state} toast={toast} />}
                    </div>
                </div>
            )}
            <StatusBar state={state} waiting={waiting.length} />
            {toastMsg && <div className="toast">{toastMsg}</div>}
        </div>
    );
}
