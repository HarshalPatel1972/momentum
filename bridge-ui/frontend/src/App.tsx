import { useState } from 'react';
import { AnimatePresence } from 'framer-motion';
import './App.css';

import WelcomeScreen from './components/WelcomeScreen';
import SourceSelect from './components/SourceSelect';
import Settings from './components/Settings';
import ChannelSelect from './components/ChannelSelect';
import ConfigPage from './components/ConfigPage';
import BridgeControl from './components/BridgeControl';

type View = 'welcome' | 'source-select' | 'settings' | 'channel-select' | 'config' | 'bridge-control';
type Channel = 'telegram' | 'whatsapp' | 'gmail' | 'sms';

function App() {
    const [currentView, setCurrentView] = useState<View>('welcome');
    const [prevView, setPrevView] = useState<View>('welcome'); // Track previous view
    const [selectedSource, setSelectedSource] = useState<'agent' | 'mcp'>('agent');
    const [selectedChannel, setSelectedChannel] = useState<Channel>('telegram');
    const [editingRecent, setEditingRecent] = useState(false);

    const handleStart = () => {
        setCurrentView('source-select');
    };

    const handleSettings = () => {
        setCurrentView('settings');
    };

    const handleBack = () => {
        setCurrentView('welcome');
    };

    const handleSourceSelect = (source: 'agent' | 'mcp') => {
        setSelectedSource(source);
        setCurrentView('channel-select');
    };

    const handleChannelSelect = (channel: Channel) => {
        setSelectedChannel(channel);
        setCurrentView('config');
    };

    const handleConfigComplete = () => {
        setPrevView(currentView);
        setCurrentView('bridge-control');
    };

    const handleBridgeStop = () => {
        setPrevView(currentView);
        setCurrentView('welcome');
    };

    const handleBridgeBack = () => {
        // Go back to previous view (config page)
        setCurrentView(prevView);
    };

    const handleRecentSelect = (channel: any) => {
        // Map config_key to Channel type
        setSelectedChannel(channel.config_key as Channel);
        setEditingRecent(true);
        setCurrentView('config');
    };

    return (
        <div id="App">
            <AnimatePresence mode="wait">
                {currentView === 'welcome' && (
                    <WelcomeScreen 
                        key="welcome"
                        onStart={handleStart} 
                        onSettings={handleSettings}
                        onRecentSelect={handleRecentSelect}
                        onViewBridge={() => setCurrentView('bridge-control')}
                    />
                )}
                {currentView === 'source-select' && (
                    <SourceSelect 
                        key="source"
                        onSelect={handleSourceSelect} 
                        onBack={handleBack} 
                    />
                )}
                {currentView === 'settings' && (
                    <Settings 
                        key="settings"
                        onBack={handleBack} 
                    />
                )}
                {currentView === 'channel-select' && (
                    <ChannelSelect
                        key="channel"
                        source={selectedSource}
                        onBack={() => setCurrentView('source-select')}
                        onSelect={handleChannelSelect}
                    />
                )}
                {currentView === 'config' && (
                    <ConfigPage
                        key="config"
                        channel={selectedChannel}
                        source={selectedSource}
                        onBack={() => setCurrentView('channel-select')}
                        onComplete={handleConfigComplete}
                    />
                )}
                {currentView === 'bridge-control' && (
                    <BridgeControl
                        key="control"
                        onStop={handleBridgeStop}
                        onBack={handleBridgeBack}
                    />
                )}
            </AnimatePresence>
        </div>
    );
}

export default App;
