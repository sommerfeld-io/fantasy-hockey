package io.sommerfeld.fantasy.hockey;

import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;

/*
 * IMPORTANT: TURN ME INTO KOTLIN !!!!!
 */

@Controller
public class AppController {

	@GetMapping("/app")
	public String app(@RequestParam(name="name", required=false, defaultValue="World") String name, Model model) {
		model.addAttribute("name", name);
		return "app";
	}
}
