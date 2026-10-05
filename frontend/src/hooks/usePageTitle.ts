import { useEffect } from 'react';

export function usePageTitle(title: string) {
  useEffect(() => {
    document.title = `${title} - Phœbus`;
    return () => { document.title = 'Phœbus'; };
  }, [title]);
}
