import type { HTMLChakraProps } from '@chakra-ui/react';
import { chakra, Center } from '@chakra-ui/react';
import React from 'react';
import type { ChangeEvent, FormEvent, FocusEvent } from 'react';

import useIsMobile from 'lib/hooks/useIsMobile';
import { Input } from 'toolkit/chakra/input';
import { InputGroup } from 'toolkit/chakra/input-group';
import { ClearButton } from 'toolkit/components/buttons/ClearButton';
import IconSvg from 'ui/shared/IconSvg';

interface Props extends Omit<HTMLChakraProps<'form'>, 'onChange'> {
  onChange?: (value: string) => void;
  onSubmit?: (event: FormEvent<HTMLFormElement>) => void;
  onBlur?: (event: FocusEvent<HTMLFormElement>) => void;
  onFocus?: () => void;
  onHide?: () => void;
  onClear?: () => void;
  onFormClick?: (event: React.MouseEvent<HTMLFormElement>) => void;
  isHeroBanner?: boolean;
  compact?: boolean;
  isSuggestOpen?: boolean;
  suggestionId?: string;
  value?: string;
  readOnly?: boolean;
}

const SearchBarInput = (
  { onChange, onSubmit, isHeroBanner, compact, isSuggestOpen, suggestionId, onFocus, onBlur, onHide, onClear, onFormClick, value, readOnly, ...rest }: Props,
  ref: React.ForwardedRef<HTMLFormElement>,
) => {
  const innerRef = React.useRef<HTMLFormElement>(null);
  React.useImperativeHandle(ref, () => innerRef.current as HTMLFormElement, []);
  const isMobile = useIsMobile();
  const isHero = Boolean(isHeroBanner && !compact);
  const [ shortcut, setShortcut ] = React.useState('Ctrl K');

  React.useEffect(() => {
    setShortcut(/Mac|iPhone|iPad/.test(navigator.platform) ? '⌘K' : 'Ctrl K');
  }, []);

  const handleChange = React.useCallback((event: ChangeEvent<HTMLInputElement>) => {
    onChange?.(event.target.value);
  }, [ onChange ]);

  const handleKeyDown = React.useCallback((event: React.KeyboardEvent<HTMLFormElement>) => {
    if (event.key === 'Escape' && isSuggestOpen && !isMobile) {
      event.preventDefault();
      innerRef.current?.querySelector('input')?.blur();
      onHide?.();
    }
  }, [ isMobile, isSuggestOpen, onHide ]);

  const startElement = (
    <IconSvg aria-hidden={ true }
      name="search"
      boxSize={ 5 }
      mx={ 2 }
    />
  );

  const endElement = (
    <>
      <ClearButton
        onClick={ onClear }
        visible={ Boolean(value?.length) }
        minH="touchTarget"
        minW="touchTarget"
        color="text.secondary"
        mx={ 2 }
      />
      { !isMobile && (
        <Center
          as="kbd"
          minW="calc(var(--chakra-spacing-10) + var(--chakra-spacing-1) / 2)"
          h="var(--chakra-spacing-5)"
          px={ 1 }
          fontSize="label"
          color="text.muted"
          whiteSpace="nowrap"
          mr={ 2 }
          borderRadius="sm"
          borderWidth="var(--chakra-border-widths-1)"
          borderColor="border.strong"
        >
          { shortcut }
        </Center>
      ) }
    </>
  );

  return (
    <chakra.form
      ref={ innerRef }
      role="search"
      aria-label="Explorer search"
      onKeyDown={ handleKeyDown }
      noValidate
      onSubmit={ onSubmit }
      onBlur={ onBlur }
      onClick={ onFormClick }
      w={ isHero ? 'calc(var(--chakra-spacing-80) * 2)' : 'var(--chakra-spacing-80)' }
      maxW="100%"
      flexShrink={ 1 }
      backgroundColor="raised"
      borderRadius={ isHero ? 'control' : 'pill' }
      position="relative"
      zIndex={ isSuggestOpen ? 'modal' : 'auto' }
      { ...rest }
    >
      <InputGroup
        startElement={ startElement }
        endElement={ endElement }
      >
        <Input
          size="md"
          name="q"
          aria-controls={ isSuggestOpen ? suggestionId : undefined }
          aria-haspopup="dialog"
          type="search"
          autoCapitalize="none"
          spellCheck={ false }
          h="auto"
          minH={ isHero ? 'calc(var(--chakra-spacing-12) + var(--chakra-spacing-1))' : 'touchTarget' }
          py={ 2 }
          borderRadius={ isHero ? 'control' : 'pill' }
          aria-label={ compact ? 'Search or ask' : 'Search' }
          placeholder={ isHero ? 'Search or ask: a transaction, address, block or token' : 'Search or ask…' }
          value={ value }
          onChange={ handleChange }
          onFocus={ onFocus }
          tabIndex={ readOnly ? -1 : 0 }
          borderWidth="var(--chakra-border-widths-1)"
          borderStyle="solid"
          borderColor="border.strong"
          color="text.primary"
          backgroundColor="raised"
          _placeholder={{ color: 'text.muted' }}
          _placeholderShown={{ borderColor: 'border.strong' }}
          _hover={{ borderColor: 'border.strong' }}
          _focus={{ borderColor: 'accent', boxShadow: 'none', _hover: { borderColor: 'accent' } }}
          _focusVisible={{
            outlineWidth: 'var(--chakra-border-widths-2)',
            outlineStyle: 'solid',
            outlineColor: 'focusRing',
            outlineOffset: 'var(--chakra-spacing-0\\.5)',
          }}
          _readOnly={{ bg: 'raised', borderColor: 'border.strong' }}
          readOnly={ readOnly }
          enterKeyHint="search"
        />
      </InputGroup>
    </chakra.form>
  );
};

export default React.memo(React.forwardRef(SearchBarInput));
