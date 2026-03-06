import { useEffect } from 'react';

export function usePageTitle(title: string) {
  useEffect(() => {
    document.title = `${title} - Phoebus`;
    return () => { document.title = 'Phoebus'; };
  }, [title]);
}
