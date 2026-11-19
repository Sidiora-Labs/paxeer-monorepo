import { Box, Text, chakra } from '@chakra-ui/react';

interface Props {
  question: string;
  answer: string;
}

const GasTrackerFaqItem = ({ question, answer }: Props) => {
  return (
    <Box as="details" borderBottomWidth="1" borderColor="hairline">
      <chakra.summary
        cursor="pointer"
        py={ 3 }
        textStyle="body"
        fontWeight="semibold"
        color="text.primary"
        _hover={{ color: 'accent' }}
        _focusVisible={{ outlineWidth: '2', outlineStyle: 'solid', outlineColor: 'focusRing', outlineOffset: 'var(--chakra-border-widths-2)' }}
      >
        { question }
      </chakra.summary>
      <Text color="text.secondary" textWrap="pretty" pb={ 4 }>{ answer }</Text>
    </Box>
  );
};

export default GasTrackerFaqItem;
