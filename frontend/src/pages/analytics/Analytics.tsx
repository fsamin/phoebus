import React from 'react';
import { Typography, Empty } from 'antd';

const Analytics: React.FC = () => {
  return (
    <div>
      <Typography.Title level={3}>Analytics</Typography.Title>
      <Empty description="Analytics will be available in a future release" />
    </div>
  );
};

export default Analytics;
