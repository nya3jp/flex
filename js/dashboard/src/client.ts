import {FlexClient} from 'flex-client';
import React, {useContext} from 'react';

export const FlexClientContext = React.createContext(new FlexClient(window.location.origin));

export function useFlexClient(): FlexClient {
  return useContext(FlexClientContext);
}
