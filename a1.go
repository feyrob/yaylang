package main

import (
	"fmt"
	"os"
	"encoding/binary"
)

func main() {
	e := main2()
	if e != nil {
		panic(e)
	}
}

type i_chip interface{
	get_size() uint64
	resolve() []byte
}

type t_chip_offset struct{
	v uint64
}
func (c t_chip_offset) get_size() uint64 {
	return c.v
}
func (c t_chip_offset) resolve() []byte{
	//fmt.Println("t_chip_offset resolve")
	return nil
}

type t_chip_uint64 struct{
	v uint64
}
func (c t_chip_uint64) get_size() uint64{
	return 8
}
func (c t_chip_uint64) resolve() []byte{
	//fmt.Println("t_chip_uint64 resolve")
	r := make([]byte, 8)
	binary.LittleEndian.PutUint64(r, c.v)
	return r
}

type t_chip_bytes struct{
	v []byte
}
func (c t_chip_bytes) get_size() uint64 {
	return uint64(len(c.v))
}
func (c t_chip_bytes) resolve() []byte {
	//fmt.Println("t_chip_bytes resolve")
	return c.v
}

type t_chip_size_sum_64 struct{
	operands []i_chip
}
func (c t_chip_size_sum_64) get_size() uint64 {
	return 8
}
func (c t_chip_size_sum_64) resolve() []byte{
	//fmt.Println("t_chip_size_sum_64 resolve")
	sum := uint64(0)
	for _, o := range c.operands{
		s := o.get_size()
		//fmt.Println("s: ", s)
		sum += s
	}
	b1 := make([]byte, 8)
	binary.LittleEndian.PutUint64(b1, sum)
	return b1
}

type t_chip_size_sum_32 struct{
	operands []i_chip
}
func (c t_chip_size_sum_32) get_size() uint64 {
	return 4
}
func (c t_chip_size_sum_32) resolve() []byte{
	sum := uint64(0)
	for _, o := range c.operands{
		s := o.get_size()
		sum += s
	}
	b1 := make([]byte, 4)
	binary.LittleEndian.PutUint32(b1, uint32(sum))
	return b1
}

type t_chip_size_sum_16 struct{
	operands []i_chip
}
func (c t_chip_size_sum_16) get_size() uint64 {
	return 2
}
func (c t_chip_size_sum_16) resolve()[]byte{
	//fmt.Println("t_chip_size_sum_16 resolve")
	sum := uint64(0)
	for _, o := range c.operands{
		sum += o.get_size()
	}
	b1 := make([]byte, 2)
	binary.LittleEndian.PutUint16(b1, uint16(sum))
	return b1
}

type t_chip_list struct{
	chips []i_chip
	name string
}
func (c t_chip_list) get_size() uint64 {
	sum := uint64(0)
	for _, v := range c.chips {
		sum += (v).get_size()
	}
	//fmt.Println("t_chip_list", c.name, "size: ", sum)
	return sum
}
func (c t_chip_list) resolve() []byte{
	//fmt.Println("t_chip_list resolve ", c.name)
	r := []byte{}
	//fmt.Println("len(c.chips): ", len(c.chips))
	for _, v := range c.chips {
		//fmt.Println("-")
		b := (v).resolve()
		r = append(r, b...)
	}
	return r
}


func create_elf_header_chips(
	virtual_base_address uint64,
	program_header_table i_chip,
	elf_header i_chip,
) []i_chip {
	// helpful docs:
	// http://www.sco.com/developers/gabi/2003-12-17/ch4.eheader.html

	chips := []i_chip{
		t_chip_bytes{
			v: []byte{
				0x7f, 0x45, 0x4c, 0x46, // magic number
				0x02, // x64
				0x01, // little endian
				0x01, // EI_VERSION
				0x01, // EI_OSABI Linux
				0x00, // EI_ABIVERSION
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // EI_PAD unused
				0x02, 0x00, // e_type ET_EXEC
				0x3e, 0x00, // e_machine x86-64
				0x01, 0x00, 0x00, 0x00, // e_version
			},
		},
		// virtual_start_address
		t_chip_size_sum_64{
			operands: []i_chip{
				t_chip_offset{v: virtual_base_address},
				elf_header,
				program_header_table,
			},
		},
		t_chip_bytes{
			v:[]byte{
				// e_phoff - physical start address of program header table
				// since the program header table follows the elf header this is equal to the elf header size
				0x40, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				// e_shoff - 0 since we don't have a section header table
				0x0, 0x0, 0x0, 0x0,  0x0, 0x0, 0x0, 0x0,
				// e_flags
				0x0, 0x0, 0x0, 0x0,
				// https://en.wikipedia.org/wiki/Executable_and_Linkable_Format
				// 64 for 64bit header (x40)
				0x40, 0x0, // e_ehsize
			},
		},
		// e_phentsize - size of program header table
		t_chip_size_sum_16{
			operands: []i_chip{
				program_header_table,
			},
		},
		t_chip_bytes{
			v:[]byte{
				0x1, 0x0, // e_phnum
				// e_shentsize - 0 since we don't have a section header table
				0x0, 0x0, // e_shentsize
				0x0, 0x0, // e_shnum
				0x0, 0x0, // e_shstrndx
			},
		},
	}
	//fmt.Println("elf_header len(chips): ", len(chips))
	return chips
}

func create_program_header_table_chips(
	virtual_base_address uint64,
	program i_chip,
) []i_chip{
	p_offset := uint64(0)
	p_align := uint64(0x1000) // 4k page

	// p_offset % p_align == p_vaddr % p_align

	//org_virt_mem_base := 0x00400040

	// helpful docs:
	// http://www.sco.com/developers/gabi/2003-12-17/ch4.eheader.html

	chips := []i_chip{
		// program header (at 0x40)
		t_chip_bytes{
			v: []byte{
				// p_type PT_LOAD
				0x1, 0x0, 0x0, 0x0,
				// p_flags execute and read
				0x5, 0x0, 0x0, 0x0,
			},
		},
		// p_offset - offset of segment (that contains start) in the virtual image
		t_chip_uint64{v:p_offset},
		// p_vaddr
		t_chip_uint64{v:virtual_base_address},
		// p_paddr - documented as ignored
		t_chip_bytes{
			v: []byte{
				0x0, 0x0, 0x0, 0x0,
				0x0, 0x0, 0x0, 0x0,
			},
		},
		// p_filesz
		t_chip_size_sum_64{
			operands: []i_chip{
				program,
			},
		},
		// p_memsz
		t_chip_size_sum_64{
			operands: []i_chip{
				program,
			},
		},

		// p_align
		t_chip_uint64{v: p_align},
	}
	return chips
}

func create_code_chips(
	virtual_base_address uint64,
	elf_header i_chip,
	program_header_table i_chip,
	code i_chip,
	str i_chip,
) []i_chip {
	chips := []i_chip{
		t_chip_bytes{
			v:[]byte{
				0xb8, // mov eax
				0x1, 0x0, 0x0, 0x0, // 1
				0xbf, // mov edi
				0x1, 0x0, 0x0, 0x0, // 1
				0x48, 0xbe, // movabs rsi
			},
		},
		t_chip_size_sum_64{
			operands: []i_chip{
				t_chip_offset{v: virtual_base_address},
				elf_header,
				program_header_table,
				code,
			},
		},
		t_chip_bytes{
			v: []byte{
				0xba, // mov edx
			},
		},
		t_chip_size_sum_32{
			operands: []i_chip{
				str,
			},
		},

		t_chip_bytes{
			v: []byte{
				0x0f, 0x05, // syscall

				0xbf, // mov edi
				0x0, 0x0, 0x0, 0x0, // exit code 0

				0xb8, // mov eax
				0x3c, 0x0 ,0x0, 0x0, // sys_exit

				0x0f, 0x05, // syscall
			},
		},
	}
	return chips
}

func main2() (error){
	//virtual_base_address := uint64(0x00000000c0ffee0000)
	//virtual_base_address := uint64(0xfec00000)

	//virtual_base_address := uint64(0xeeffc00000)
	//v := []byte {
		//0x0, 0x0, 0xc0, 0xff,
		//0xee, 0x0, 0x0, 0x0,
	//}
	//v := []byte {
		//0x0, 0x0, 0xc0, 0xff,
		//0xee, 0x7f, 0x0, 0x0,
	//}
	//v := []byte {
		//0x0, 0x0, 0xaa, 0xaa,
		//0xaa, 0x0a, 0x0, 0x0,
	//}
	//v := []byte {
		//0x0, 0x0, 0x0, 0x0,
		//0x0, 0x0a, 0x0, 0x0,
	//}

	// 79 - y
	// 61 - a
	//v := []byte {
		//0x0, 0x0, 'y', 'a',
		//'y', '!', 0x0, 0x0,
	//}
	v := []byte {
		0x0, 0x0, 0x0, 'y',
		'a', 'y', 0x0, 0x0,
	}
	//v := []byte {
		//0x0, 0x0, 0x0, '^',
		//'-', '^', 0x0, 0x0,
	//}
	//v := []byte {
		//0x0, 0x0, 0x0, '1',
		//'2', '3', 0x0, 0x0,
	//}
	//v := []byte {
		//0x0, 0x0, 0x0, 0x0,
		//'y', 'l', 0x0, 0x0,
	//}
	virtual_base_address := binary.LittleEndian.Uint64(v)


	elf_header := &t_chip_list{name: "elf_header"}
	program_header_table := &t_chip_list{name: "program_header_table"}
	code := &t_chip_list{name: "code"}
	my_string := &t_chip_list{name: "my_string"}

	program := &t_chip_list{
		chips: []i_chip{
			elf_header,
			program_header_table,
			code,
			my_string,
		},
	}

	elf_header.chips = create_elf_header_chips(
		virtual_base_address,
		program_header_table,
		elf_header,
	)

	//fmt.Println("len(elf_header.chips): ", len(elf_header.chips))
	//fmt.Println("y: ", len(program.chips[0].(*t_chip_list).chips) )

	program_header_table.chips = create_program_header_table_chips(
		virtual_base_address,
		program,
	)
	code.chips = create_code_chips(
		virtual_base_address,
		elf_header,
		program_header_table,
		code,
		my_string,
	)
	my_string.chips = []i_chip{
		t_chip_bytes{
			v: []byte(
				"kthxbye!\n",
			),
		},
	}

	f, err := os.Create("x")
	if err != nil {
		return err
	}
	defer f.Close()

	b0 := program.resolve()

	fmt.Println("program size: ", len(b0))

	f.Write(b0)

	if err != nil {
		return err
	}
	f.Sync()

	return nil
}

