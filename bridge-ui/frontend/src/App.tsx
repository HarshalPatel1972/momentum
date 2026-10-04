import { useCallback, useEffect, useRef, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { Activity as ActivityIcon, House, Info, Plug, Settings as SettingsIcon } from 'lucide-react';
import { EventsOn } from '../wailsjs/runtime';
import { Activity, api, AppState, Brand } from './lib';
import Welcome from './components/Welcome';
import Setup from './components/Setup';
import Home from './components/Home';
import { AboutPage, ActivityPage, IdesPage, SettingsPage } from './components/Pages';

type Mode = 'loading' | 'welcome' | 'setup' | 'main';
type Page = 'home' | 'activity' | 'ides' | 'settings' | 'about';

const NAV: { id: Page; label: string; icon: typeof House }[] = [
    { id: 'home', label: 'Home', icon: House },
    { id: 'activity', label: 'Activity', icon: ActivityIcon },
    { id: 'ides', label: 'IDEs', icon: Plug },
    { id: 'settings', label: 'Settings', icon: SettingsIcon },
    { id: 'about', label: 'About', icon: Info },
];

export default function App() {
    const [mode, setMode] = useState<Mode>('loading');
    const [page, setPage] = useState<Page>('home');
    const [state, setState] = useState<AppState | null>(null);
    const [activity, setActivity] = useState<Activity[]>([]);
    const [toastMsg, setToastMsg] = useState('');
    const toastTimer = useRef<number>();

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

    useEffect(() => {
        // "#setup-2", "#welcome" or "#<page>" opens a screen directly (previews/screenshots).
        const hash = window.location.hash.slice(1);
        refresh().then(s => {
            if (hash === 'welcome' || hash.startsWith('setup')) return setMode(hash === 'welcome' ? 'welcome' : 'setup');
            if (NAV.some(n => n.id === hash)) setPage(hash as Page);
            setMode(s.configured ? 'main' : 'welcome');
        });
        const offs = [
            EventsOn('activity', () => api.activity().then(a => setActivity(a || []))),
            EventsOn('state', () => refresh()),
        ];
        // Keep the status honest if a background hub starts or stops.
        const poll = window.setInterval(refresh, 8000);
        return () => { offs.forEach(off => off()); window.clearInterval(poll); };
    }, [refresh]);

    const waiting = activity.filter(a => a.state === 'waiting').length;

    if (mode === 'loading' || !state) return <div style={{ height: '100%', background: 'var(--bg)' }} />;
    if (mode === 'welcome') return <Welcome onStart={() => setMode('setup')} />;
    if (mode === 'setup') return <Setup initialStep={Number(window.location.hash.split('-')[1] || 1) - 1} onBack={() => setMode('welcome')} onFinish={() => { refresh(); setMode('main'); setPage('home'); }} />;

    return (
        <div className="shell">
            <aside className="sidebar">
                <Brand />
                <nav className="nav">
                    {NAV.map(n => (
                        <button key={n.id} className={page === n.id ? 'active' : ''} onClick={() => setPage(n.id)}>
                            <n.icon size={17} /> {n.label}
                            {n.id === 'activity' && waiting > 0 && <span className="count">{waiting}</span>}
                        </button>
                    ))}
                </nav>
                <div className="side-foot">
                    <div className="side-status">
                        <span className={`dot ${!state.running ? 'bad' : state.atDesk ? 'warn' : 'ok live'}`} />
                        <div>
                            <b>{!state.running ? 'Not running' : state.atDesk ? 'At your desk' : 'Away mode on'}</b>
                            <span className="faint">{!state.running ? (state.problem || 'Open Settings') : state.atDesk ? 'Agents ask in chat' : 'Questions go to your phone'}</span>
                        </div>
                    </div>
                    <div className="side-ver">Momentum {state.version}</div>
                </div>
            </aside>

            <AnimatePresence mode="wait">
                <motion.div key={page} style={{ minHeight: 0, minWidth: 0, display: 'flex', flexDirection: 'column' }}
                    initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }} transition={{ duration: .18 }}>
                    {page === 'home' && <Home state={state} activity={activity} go={setPage} toast={toast} refresh={refresh} />}
                    {page === 'activity' && <ActivityPage activity={activity} toast={toast} />}
                    {page === 'ides' && <IdesPage refresh={refresh} toast={toast} />}
                    {page === 'settings' && <SettingsPage state={state} refresh={refresh} toast={toast} />}
                    {page === 'about' && <AboutPage state={state} toast={toast} />}
                </motion.div>
            </AnimatePresence>

            <AnimatePresence>
                {toastMsg && (
                    <motion.div className="toast" initial={{ opacity: 0, y: 12, x: '-50%' }} animate={{ opacity: 1, y: 0, x: '-50%' }} exit={{ opacity: 0, y: 12, x: '-50%' }}>
                        {toastMsg}
                    </motion.div>
                )}
            </AnimatePresence>
        </div>
    );
}
