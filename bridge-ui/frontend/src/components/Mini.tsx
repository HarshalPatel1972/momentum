import { Maximize2, X } from 'lucide-react';
import { Activity } from '../lib';
import { PagerDevice } from './Pager';

/** The mini pager: an always-on-top widget in the corner of the screen. Drag it anywhere. */
export default function Mini({ page, onExpand, onClose, onAnswered }: {
    page?: Activity;
    onExpand: () => void;
    onClose: () => void;
    onAnswered: () => void;
}) {
    return (
        <div className="mini">
            <PagerDevice
                compact
                page={page}
                idle={{ top: 'ON CALL', main: 'NO PAGES' }}
                onAnswered={onAnswered}
                extra={<span className="mini-ctl">
                    <button title="Open the full window (Ctrl+M)" onClick={onExpand}><Maximize2 size={11} /></button>
                    <button title="Close to tray" onClick={onClose}><X size={12} /></button>
                </span>}
            />
        </div>
    );
}
