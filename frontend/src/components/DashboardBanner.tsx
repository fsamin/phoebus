import React from 'react';
import { Alert } from 'antd';
import MarkdownRenderer from './MarkdownRenderer';
import type { BannerLevel } from '../api/client';

interface DashboardBannerProps {
  level: BannerLevel;
  message_md: string;
}

// The API speaks the vocabulary of the content admonitions (info/warning/danger)
// while Ant Design's Alert expects success/info/warning/error. This mapping is
// the single place where the two meet — the admin preview renders this same
// component so the two views cannot drift apart.
const alertType = { info: 'info', warning: 'warning', danger: 'error' } as const;

const DashboardBanner: React.FC<DashboardBannerProps> = ({ level, message_md }) => (
  <Alert
    type={alertType[level] ?? 'info'}
    showIcon
    style={{ marginBottom: 24 }}
    // Markdown goes in `description`: `message` is styled as a single bold
    // heading line and block content overflows it.
    description={<MarkdownRenderer content={message_md} />}
  />
);

export default DashboardBanner;
