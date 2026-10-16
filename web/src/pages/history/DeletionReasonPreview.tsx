import { useState } from 'react';
import { Button, Modal } from 'antd';
import './deletion-reason-preview.css';

export function DeletionReasonPreview({ reason }: { reason: string }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="deletion-reason-preview">
      <p className="deletion-reason-preview__summary">{reason}</p>
      <Button type="link" onClick={() => setOpen(true)}>
        查看原因
      </Button>
      <Modal title="删除原因" open={open} onCancel={() => setOpen(false)} footer={null} width={600}>
        <div className="deletion-reason-preview__full">{reason}</div>
      </Modal>
    </div>
  );
}
