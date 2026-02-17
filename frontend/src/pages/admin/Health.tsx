import React from 'react';
import { Card, Typography, Tag, Row, Col } from 'antd';
import { CheckCircleOutlined } from '@ant-design/icons';

const Health: React.FC = () => {
  return (
    <div>
      <Typography.Title level={3}>Platform Health</Typography.Title>
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={8}>
          <Card>
            <Card.Meta
              avatar={<CheckCircleOutlined style={{ fontSize: 32, color: '#52c41a' }} />}
              title="API"
              description={<Tag color="green">Healthy</Tag>}
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card>
            <Card.Meta
              avatar={<CheckCircleOutlined style={{ fontSize: 32, color: '#52c41a' }} />}
              title="Database"
              description={<Tag color="green">Connected</Tag>}
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card>
            <Card.Meta
              avatar={<CheckCircleOutlined style={{ fontSize: 32, color: '#52c41a' }} />}
              title="Sync Worker"
              description={<Tag color="green">Running</Tag>}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Health;
